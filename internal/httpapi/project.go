package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/terrain"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const projectSchema = 4
const projectFieldEncoding = "float64 little-endian, field-major then row-major; edge chunks clipped to dimensions"
const maxProjectBytes = 256 << 20
const maxExpandedBytes = 1 << 30

type projectFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type projectRegion struct {
	ID     string `json:"id"`
	Parent string `json:"parent"`
	Layer  string `json:"layer"`
	Level  int    `json:"level"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	File   string `json:"file"`
}
type projectManifest struct {
	Climate         *climateArchive `json:"climate,omitempty"`
	AuthoredClimate *climateArchive `json:"authoredClimate,omitempty"`
	GeologyFile     string          `json:"geologyFile,omitempty"`
	ResourcesFile   string          `json:"resourcesFile,omitempty"`
	Format          string          `json:"format"`
	Schema          int             `json:"schema"`
	Engine          string          `json:"detailEngine"`
	ID              string          `json:"worldId"`
	Width           int             `json:"width"`
	Height          int             `json:"height"`
	WrapX           bool            `json:"wrapX"`
	Coordinates     string          `json:"coordinates"`
	ElevationUnits  string          `json:"elevationUnits"`
	FieldEncoding   string          `json:"fieldEncoding"`
	Generation      json.RawMessage `json:"generation"`
	Fields          []string        `json:"fields"`
	ChunkSize       int             `json:"chunkSize"`
	Files           []projectFile   `json:"files"`
	Regions         []projectRegion `json:"regions"`
}
type projectEdits struct {
	Current    terrain.Snapshot  `json:"current"`
	Operations []json.RawMessage `json:"operations"`
}

func (v *session) initStorage() error {
	if v.dir != "" {
		return nil
	}
	if v.cacheRoot == "" {
		v.cacheRoot = filepath.Join(".tools", "projects")
	}
	if err := os.MkdirAll(v.cacheRoot, 0700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(v.cacheRoot, "world-")
	if err != nil {
		return err
	}
	v.dir = dir
	v.tiles = map[string]bool{}
	return nil
}
func detailKey(l, x, y int) string { return fmt.Sprintf("%d/%d/%d", l, x, y) }
func (v *session) putDetail(key string, data []byte) error {
	if err := v.initStorage(); err != nil {
		return err
	}
	filename := filepath.Join(v.dir, filepath.FromSlash(key+".json"))
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return err
	}
	v.tiles[key] = true
	return nil
}
func (v *session) storedDetail(l, x, y int) ([]byte, error) {
	if l < 0 || l > terrain.DetailLevels || x < 0 || y < 0 || float64(x)*32/math.Exp2(float64(l)) >= float64(v.solver.W-1) || float64(y)*32/math.Exp2(float64(l)) >= float64(v.solver.H-1) {
		return nil, fmt.Errorf("detail tile outside world or supported levels")
	}
	key := detailKey(l, x, y)
	if v.tiles[key] {
		return os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(key+".json")))
	}
	if v.detail == nil {
		v.detail = terrain.NewDetailModel(v.solver.Environment)
	}
	// Persist ancestors as well, so every region reference resolves in the file.
	if l > 0 {
		if _, err := v.storedDetail(l-1, x/2, y/2); err != nil {
			return nil, err
		}
	}
	tile, err := v.detail.Tile(l, x, y)
	if err != nil {
		return nil, err
	}
	if l > 0 {
		parentBytes, err := v.storedDetail(l-1, x/2, y/2)
		if err != nil {
			return nil, err
		}
		var parent terrain.DetailTile
		if err = json.Unmarshal(parentBytes, &parent); err != nil {
			return nil, err
		}
		inheritStoredParent(&tile, parent)
	}
	data, err := json.Marshal(tile)
	if err != nil {
		return nil, err
	}
	if err = v.putDetail(key, data); err != nil {
		return nil, err
	}
	return data, nil
}

// When a previously stored parent differs from the procedural model, missing
// children inherit its actual surface. Saved detail remains authoritative even
// while exploring new descendants. Existing tiles never pass through this path.
func inheritStoredParent(tile *terrain.DetailTile, parent terrain.DetailTile) {
	correction := make([]float64, len(tile.Points))
	for y := 0; y < 33; y++ {
		for x := 0; x < 33; x++ {
			u, v := float64(tile.X%2*16)+float64(x)/2, float64(tile.Y%2*16)+float64(y)/2
			ix, iy := int(u), int(v)
			a, b := u-float64(ix), v-float64(iy)
			z := func(dx, dy int) float64 { return parent.Points[min(32, iy+dy)*33+min(32, ix+dx)].Elevation }
			target := 0.
			if a+b <= 1 {
				target = z(0, 0)*(1-a-b) + z(1, 0)*a + z(0, 1)*b
			} else {
				target = z(1, 1)*(a+b-1) + z(1, 0)*(1-b) + z(0, 1)*(1-a)
			}
			i := y*33 + x
			p := &tile.Points[i]
			correction[i] = target - p.Parent
			p.Elevation += correction[i]
			p.Parent = target
			if x%2 == 0 && y%2 == 0 {
				p.Elevation = z(0, 0)
			}
			if p.WaterBody > 0 {
				p.WaterDepth = math.Max(0, p.WaterLevel-p.Elevation)
			}
			if p.RiverDepth > 0 {
				p.RiverDepth = math.Max(0, p.RiverLevel-p.Elevation)
			}
		}
	}
	for y := 0; y < 33; y++ {
		for x := 0; x < 33; x++ {
			dx := (correction[y*33+min(32, x+1)] - correction[y*33+max(0, x-1)]) / (float64(min(32, x+1)-max(0, x-1)) * tile.Step)
			dy := (correction[min(32, y+1)*33+x] - correction[max(0, y-1)*33+x]) / (float64(min(32, y+1)-max(0, y-1)) * tile.Step)
			p := &tile.Points[y*33+x]
			p.Gradient[0] += dx
			p.Gradient[1] += dy
			p.Gradient[2] += dx
			p.Gradient[3] += dy
		}
	}
}

func (s *Server) saveProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UI json.RawMessage `json:"ui"`
	}
	if err := decode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	s.mu.Unlock()
	if v == nil {
		fail(w, 404, "Map session not found")
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.last = time.Now()
	if v.solver.Status != "done" {
		fail(w, 409, "Finish generating the world before saving its project.")
		return
	}
	if v.retired {
		fail(w, 409, "World was deleted or replaced")
		return
	}
	if !sameJSON(v.projectUI, req.UI) {
		v.unsaved = true
	}
	v.projectUI = req.UI
	data, err := encodeProject(v)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="KRIEMHILD.world.zip"`)
	w.Write(data)
}

func encodeProject(v *session) ([]byte, error) {
	s := v.solver
	if v.worldID == "" {
		v.worldID = token()
	}
	m := projectManifest{Format: "KRIEMHILD World Project", Schema: projectSchema, Engine: terrain.DetailEngineVersion, ID: v.worldID, Width: s.W, Height: s.H, WrapX: s.WrapX, Coordinates: "parent-grid x east, y south; globe longitude=360*x/(width-1), colatitude=180*y/(height-1)", ElevationUnits: "metres; local water surfaces stored explicitly", FieldEncoding: projectFieldEncoding, Generation: v.generation, ChunkSize: 32, Files: []projectFile{}, Regions: []projectRegion{}}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	expanded := 0
	add := func(name string, data []byte) error {
		expanded += len(data)
		if len(data) > 16<<20 || expanded > maxExpandedBytes || len(m.Files) >= 65535 {
			return fmt.Errorf("world project exceeds the supported archive size; no incomplete project was saved")
		}
		sum := sha256.Sum256(data)
		m.Files = append(m.Files, projectFile{name, hex.EncodeToString(sum[:]), len(data)})
		writer, err := z.Create("KRIEMHILD/" + name)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	}
	addJSON := func(name string, value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return add(name, data)
	}
	copySolver := *s
	copySolver.Rules = nil
	copySolver.Environment = nil
	if v.base != nil {
		copySolver.Dom = v.base.Dom
		copySolver.Locked = v.base.Locked
		copySolver.Pinned = v.base.Pinned
	}
	if err := addJSON("base/solver.json", copySolver); err != nil {
		return nil, err
	}
	if err := addJSON("base/config.json", s.Rules.Config); err != nil {
		return nil, err
	}
	if err := addJSON("edits/world.json", projectEdits{s.Snapshot(), v.edits}); err != nil {
		return nil, err
	}
	if err := addJSON("world/project.json", v.authored()); err != nil {
		return nil, err
	}
	if len(v.projectUI) == 0 {
		v.projectUI = json.RawMessage(`{}`)
	}
	if err := add("view/builder.json", v.projectUI); err != nil {
		return nil, err
	}
	if s.Environment != nil {
		e := *s.Environment
		e.Fields = nil
		var climateErr error
		m.Climate, climateErr = writeClimateArchive(e.Climate, "climate/base", addJSON)
		if climateErr != nil {
			return nil, climateErr
		}
		if v.climateLayer() != nil && v.climate.State != nil {
			m.AuthoredClimate, climateErr = writeClimateArchive(v.climate.State, "climate/authored", addJSON)
			if climateErr != nil {
				return nil, climateErr
			}
			m.AuthoredClimate.Signature = v.climate.Signature
		}
		e.Climate = nil
		if err := s.Environment.ValidateArchipelagos(); err != nil {
			return nil, err
		}
		if err := s.Environment.ValidateNaturalData(); err != nil {
			return nil, err
		}
		if e.Geology != nil {
			m.GeologyFile, m.ResourcesFile = "natural/geology.json", "natural/resources.json"
			if err := addJSON(m.GeologyFile, e.Geology); err != nil {
				return nil, err
			}
			if err := addJSON(m.ResourcesFile, e.Resources); err != nil {
				return nil, err
			}
			e.Geology, e.Resources = nil, nil
		}
		if err := addJSON("base/environment.json", e); err != nil {
			return nil, err
		}
		for name := range s.Environment.Fields {
			m.Fields = append(m.Fields, name)
		}
		sort.Strings(m.Fields)
		for y := 0; y < s.H; y += 32 {
			for x := 0; x < s.W; x += 32 {
				var data bytes.Buffer
				for _, name := range m.Fields {
					for yy := y; yy < min(s.H, y+32); yy++ {
						for xx := x; xx < min(s.W, x+32); xx++ {
							binary.Write(&data, binary.LittleEndian, s.Environment.Fields[name][yy*s.W+xx])
						}
					}
				}
				file := fmt.Sprintf("base/fields/%d/%d.f64", x/32, y/32)
				if err := add(file, data.Bytes()); err != nil {
					return nil, err
				}
				m.Regions = append(m.Regions, projectRegion{ID: fmt.Sprintf("base/%d/%d", x/32, y/32), Parent: "world", Layer: "base", Level: 0, X: x / 32, Y: y / 32, File: file})
			}
		}
		if v.detail == nil {
			v.detail = terrain.NewDetailModel(s.Environment)
		}
		if err := addJSON("detail/model.json", v.detail.ProjectState()); err != nil {
			return nil, err
		}
		keys := []string{}
		for k := range v.tiles {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			data, err := os.ReadFile(filepath.Join(v.dir, filepath.FromSlash(key+".json")))
			if err != nil {
				return nil, err
			}
			file := "detail/tiles/" + key + ".json"
			if err := add(file, data); err != nil {
				return nil, err
			}
			var l, x, y int
			fmt.Sscanf(key, "%d/%d/%d", &l, &x, &y)
			parent := fmt.Sprintf("base/%d/%d", x, y)
			if l > 0 {
				parent = "detail/" + detailKey(l-1, x/2, y/2)
			}
			m.Regions = append(m.Regions, projectRegion{"detail/" + key, parent, "generated-detail", l, x, y, file})
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	writer, err := z.Create("KRIEMHILD/manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(data); err != nil {
		return nil, err
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	if archive.Len() > maxProjectBytes {
		return nil, fmt.Errorf("world ZIP exceeds the 256 MiB portable-project limit")
	}
	return archive.Bytes(), nil
}

func safeProjectPath(name string) bool {
	return name != "" && name == path.Clean(name) && !strings.HasPrefix(name, "/") && !strings.Contains(name, "\\") && !strings.Contains(name, ":") && name != ".." && !strings.HasPrefix(name, "../")
}

// Archive and folder imports share one reader. References are resolved relative
// to the unique manifest, never flattened by filename or extracted blindly.
func readProjectFiles(r *http.Request) (map[string][]byte, error) {
	files := map[string][]byte{}
	total := 0
	add := func(name string, data []byte) error {
		if !safeProjectPath(name) || files[name] != nil {
			return fmt.Errorf("unsafe or duplicate project path %q", name)
		}
		total += len(data)
		if total > maxExpandedBytes || len(files) >= 65536 {
			return fmt.Errorf("project exceeds import limits")
		}
		files[name] = data
		return nil
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		reader, err := r.MultipartReader()
		if err != nil {
			return nil, err
		}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			// Form field name retains webkitRelativePath, unlike sanitized filenames.
			data, err := io.ReadAll(io.LimitReader(part, 16<<20+1))
			part.Close()
			if err != nil {
				return nil, err
			}
			if len(data) > 16<<20 {
				return nil, fmt.Errorf("project entry too large")
			}
			if err = add(part.FormName(), data); err != nil {
				return nil, err
			}
		}
	} else {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("not a valid world project ZIP")
		}
		for _, f := range archive.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if f.Mode()&os.ModeSymlink != 0 || f.UncompressedSize64 > 16<<20 {
				return nil, fmt.Errorf("unsupported project entry")
			}
			stream, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(io.LimitReader(stream, 16<<20+1))
			stream.Close()
			if err != nil {
				return nil, err
			}
			if len(data) > 16<<20 {
				return nil, fmt.Errorf("project entry too large")
			}
			if err = add(f.Name, data); err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

func decodeProject(files map[string][]byte, cacheRoot ...string) (*session, error) {
	manifest := ""
	for name := range files {
		if path.Base(name) == "manifest.json" {
			if manifest != "" {
				return nil, fmt.Errorf("project must have one root manifest")
			}
			manifest = name
		}
	}
	if manifest == "" {
		return nil, fmt.Errorf("world project manifest missing")
	}
	var m projectManifest
	if err := json.Unmarshal(files[manifest], &m); err != nil {
		return nil, err
	}
	if m.Format != "KRIEMHILD World Project" || (m.Schema < 1 || m.Schema > projectSchema) || m.Engine != terrain.DetailEngineVersion || m.ChunkSize != 32 || m.FieldEncoding != projectFieldEncoding || m.ID == "" {
		return nil, fmt.Errorf("unsupported project schema or detail engine version")
	}
	prefix := strings.TrimSuffix(manifest, "manifest.json")
	data := map[string][]byte{}
	for _, ref := range m.Files {
		if !safeProjectPath(ref.Path) || data[ref.Path] != nil {
			return nil, fmt.Errorf("invalid manifest file reference")
		}
		value, ok := files[prefix+ref.Path]
		if !ok || len(value) != ref.Bytes {
			return nil, fmt.Errorf("missing or incomplete file %s", ref.Path)
		}
		sum := sha256.Sum256(value)
		if hex.EncodeToString(sum[:]) != ref.SHA256 {
			return nil, fmt.Errorf("checksum mismatch: %s", ref.Path)
		}
		data[ref.Path] = value
	}
	if len(files) != len(m.Files)+1 {
		return nil, fmt.Errorf("unreferenced files in project")
	}
	decode := func(name string, v any) error {
		b, ok := data[name]
		if !ok {
			return fmt.Errorf("required file missing: %s", name)
		}
		return json.Unmarshal(b, v)
	}
	var solver terrain.Solver
	if m.Schema >= 2 && data["world/project.json"] == nil {
		return nil, fmt.Errorf("authored world data missing")
	}
	var config terrain.Config
	var edits projectEdits
	if err := decode("base/solver.json", &solver); err != nil {
		return nil, err
	}
	if err := decode("base/config.json", &config); err != nil {
		return nil, err
	}
	if err := decode("edits/world.json", &edits); err != nil {
		return nil, err
	}
	if solver.W != m.Width || solver.H != m.Height || solver.WrapX != m.WrapX || m.Width < 16 || m.Width > 256 || m.Height < 16 || m.Height > 256 {
		return nil, fmt.Errorf("manifest dimensions disagree with world")
	}
	base := solver.Snapshot()
	var env *terrain.Environment
	if data["base/environment.json"] == nil && (m.GeologyFile != "" || m.ResourcesFile != "" || data["natural/geology.json"] != nil || data["natural/resources.json"] != nil) {
		return nil, fmt.Errorf("natural layers have no base terrain")
	}
	if data["base/environment.json"] != nil {
		env = &terrain.Environment{}
		if err := decode("base/environment.json", env); err != nil {
			return nil, err
		}
		if env.Climate != nil {
			return nil, fmt.Errorf("climate must use manifest-referenced chunks")
		}
		if m.GeologyFile != "" || m.ResourcesFile != "" {
			if m.Schema < 3 || m.GeologyFile != "natural/geology.json" || m.ResourcesFile != "natural/resources.json" || env.Geology != nil || env.Resources != nil {
				return nil, fmt.Errorf("invalid natural layer manifest references")
			}
			env.Geology = &terrain.GeologicalState{}
			env.Resources = &terrain.ResourceState{}
			if err := decode(m.GeologyFile, env.Geology); err != nil {
				return nil, err
			}
			if err := decode(m.ResourcesFile, env.Resources); err != nil {
				return nil, err
			}
		} else if data["natural/geology.json"] != nil || data["natural/resources.json"] != nil {
			return nil, fmt.Errorf("unreferenced natural layers")
		}
		env.Fields = map[string][]float64{}
		if len(m.Fields) < 1 || len(m.Fields) > 256 {
			return nil, fmt.Errorf("invalid field inventory")
		}
		for _, name := range m.Fields {
			if name == "" || env.Fields[name] != nil {
				return nil, fmt.Errorf("duplicate field")
			}
			env.Fields[name] = make([]float64, m.Width*m.Height)
		}
		for y := 0; y < m.Height; y += 32 {
			for x := 0; x < m.Width; x += 32 {
				name := fmt.Sprintf("base/fields/%d/%d.f64", x/32, y/32)
				b, ok := data[name]
				expected := len(m.Fields) * min(32, m.Width-x) * min(32, m.Height-y) * 8
				if !ok || len(b) != expected {
					return nil, fmt.Errorf("incomplete field chunk %s", name)
				}
				at := 0
				for _, name := range m.Fields {
					for yy := y; yy < min(m.Height, y+32); yy++ {
						for xx := x; xx < min(m.Width, x+32); xx++ {
							env.Fields[name][yy*m.Width+xx] = math.Float64frombits(binary.LittleEndian.Uint64(b[at : at+8]))
							at += 8
						}
					}
				}
			}
		}
	}
	climateFiles := map[string]bool{}
	if m.Climate != nil || m.AuthoredClimate != nil {
		if m.Schema < 4 || env == nil || env.Climate != nil || m.Climate == nil {
			return nil, fmt.Errorf("invalid climate layer references")
		}
		var err error
		env.Climate, err = readClimateArchive(m.Climate, "climate/base", m.Width, m.Height, data, climateFiles)
		if err != nil {
			return nil, err
		}
	}
	if err := terrain.RestoreProjectSolver(&solver, config, env, edits.Current); err != nil {
		return nil, err
	}
	v := &session{solver: &solver, base: &base, edits: edits.Operations, generation: m.Generation, projectUI: data["view/builder.json"], worldID: m.ID, last: time.Now(), undo: map[string]terrain.Snapshot{}}
	if b := data["world/project.json"]; b != nil {
		v.world = &world.State{}
		if err := json.Unmarshal(b, v.world); err != nil {
			return nil, err
		}
		if err := v.world.Validate(solver.W, solver.H); err != nil {
			return nil, err
		}
	}
	if m.AuthoredClimate != nil {
		state, err := readClimateArchive(m.AuthoredClimate, "climate/authored", m.Width, m.Height, data, climateFiles)
		if err != nil {
			return nil, err
		}
		if len(v.authored().Operations) == 0 || m.AuthoredClimate.Signature != v.climateKey() {
			return nil, fmt.Errorf("authored climate does not match saved terrain edits")
		}
		v.climate = authoredClimate{m.AuthoredClimate.Signature, state}
	}
	for file := range data {
		if strings.HasPrefix(file, "climate/") && !climateFiles[file] {
			return nil, fmt.Errorf("unreferenced climate data")
		}
	}
	if len(cacheRoot) > 0 {
		v.cacheRoot = cacheRoot[0]
	}
	if len(v.projectUI) == 0 || !json.Valid(v.projectUI) {
		return nil, fmt.Errorf("invalid builder metadata")
	}
	if env != nil {
		var state terrain.DetailState
		if err := decode("detail/model.json", &state); err != nil {
			return nil, err
		}
		model, err := terrain.RestoreDetailModel(env, state)
		if err != nil {
			return nil, err
		}
		v.detail = model
	}
	regions := map[string]projectRegion{}
	tileData := map[string][]byte{}
	decodedTiles := map[string]terrain.DetailTile{}
	for _, region := range m.Regions {
		if regions[region.ID].ID != "" || data[region.File] == nil {
			return nil, fmt.Errorf("duplicate region or missing data")
		}
		regions[region.ID] = region
		if region.Layer == "base" {
			if region.ID != fmt.Sprintf("base/%d/%d", region.X, region.Y) || region.Parent != "world" || region.Level != 0 || region.File != fmt.Sprintf("base/fields/%d/%d.f64", region.X, region.Y) {
				return nil, fmt.Errorf("invalid base region")
			}
			continue
		}
		if region.Layer != "generated-detail" {
			return nil, fmt.Errorf("unsupported region layer")
		}
		key := detailKey(region.Level, region.X, region.Y)
		if region.ID != "detail/"+key || region.File != "detail/tiles/"+key+".json" {
			return nil, fmt.Errorf("detail identity mismatch")
		}
		parent := fmt.Sprintf("base/%d/%d", region.X, region.Y)
		if region.Level > 0 {
			parent = "detail/" + detailKey(region.Level-1, region.X/2, region.Y/2)
		}
		if region.Parent != parent {
			return nil, fmt.Errorf("invalid detail parent")
		}
		var tile terrain.DetailTile
		if err := json.Unmarshal(data[region.File], &tile); err != nil {
			return nil, err
		}
		if tile.Level != region.Level || tile.X != region.X || tile.Y != region.Y {
			return nil, fmt.Errorf("tile disagrees with manifest")
		}
		if err := terrain.ValidateDetailTile(tile, env); err != nil {
			return nil, err
		}
		tileData[key] = data[region.File]
		decodedTiles[region.ID] = tile
	}
	for _, region := range regions {
		if region.Parent != "world" && regions[region.Parent].ID == "" {
			return nil, fmt.Errorf("missing parent region")
		}
	}
	if env != nil {
		for y := 0; y < m.Height; y += 32 {
			for x := 0; x < m.Width; x += 32 {
				if regions[fmt.Sprintf("base/%d/%d", x/32, y/32)].ID == "" {
					return nil, fmt.Errorf("missing base region")
				}
			}
		}
	}
	for name := range data {
		if strings.HasPrefix(name, "detail/tiles/") {
			key := strings.TrimSuffix(strings.TrimPrefix(name, "detail/tiles/"), ".json")
			if tileData[key] == nil {
				return nil, fmt.Errorf("unreferenced detail tile")
			}
		}
	}
	for id, tile := range decodedTiles {
		for y := 0; y < 33; y++ {
			for x := 0; x < 33; x++ {
				p := tile.Points[y*33+x]
				if tile.Level == 0 {
					cell := min(m.Height-1, tile.Y*32+y)*m.Width + min(m.Width-1, tile.X*32+x)
					if p.Elevation != env.Fields["elevation"][cell] {
						return nil, fmt.Errorf("root detail contradicts stored world")
					}
				} else if x%2 == 0 && y%2 == 0 {
					parent := decodedTiles[regions[id].Parent]
					px, py := (tile.X%2)*16+x/2, (tile.Y%2)*16+y/2
					if len(parent.Points) != 1089 || p.Elevation != parent.Points[py*33+px].Elevation {
						return nil, fmt.Errorf("detail contradicts parent elevation")
					}
				}
			}
		}
	}
	// Validate hierarchy before installing anything. Stored tiles are authoritative;
	// their data is copied intact, not compared to a newly generated replacement.
	for key, b := range tileData {
		if err := v.putDetail(key, b); err != nil {
			if v.dir != "" {
				os.RemoveAll(v.dir)
			}
			return nil, err
		}
	}
	return v, nil
}

func (s *Server) importProject(w http.ResponseWriter, r *http.Request) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	r.Body = http.MaxBytesReader(w, r.Body, maxProjectBytes)
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		fail(w, 429, "Project import is busy; retry shortly.")
		return
	}
	files, err := readProjectFiles(r)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	v, err := decodeProject(files, s.cacheRoot)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	v.unsaved, v.dirty = true, true
	_ = s.replaceWorld(v.worldID, func() error { return nil })

	s.mu.Lock()
	if len(s.sessions) >= 32 {
		s.mu.Unlock()
		if v.dir != "" {
			os.RemoveAll(v.dir)
		}
		fail(w, 429, "Too many active maps")
		return
	}
	id := token()
	s.sessions[id] = v
	s.mu.Unlock()
	state := sessionState(v)
	state["id"] = id
	state["projectUI"] = v.projectUI
	state["storedDetailCount"] = len(v.tiles)
	respond(w, 201, state)
}
