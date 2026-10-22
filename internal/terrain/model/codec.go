package model

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
)

// Compact field storage is independent of the current generator. Loading never
// runs generation. Project archives checksum each metadata/field payload.
func (p *Planet) Encode() ([]byte, []byte, error) {
	meta, err := json.Marshal(p)
	if err != nil {
		return nil, nil, err
	}
	var b bytes.Buffer
	keys := make([]string, 0, len(p.Grid.Fields))
	for k := range p.Grid.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("KTF1")
	binary.Write(&b, binary.LittleEndian, uint32(len(keys)))
	for _, k := range keys {
		binary.Write(&b, binary.LittleEndian, uint16(len(k)))
		b.WriteString(k)
		binary.Write(&b, binary.LittleEndian, p.Grid.Fields[k])
	}
	if p.Surface != nil {
		b.WriteString("KTS1")
		for _, a := range [][]float32{p.Surface.Bed, p.Surface.Coast, p.Surface.Roughness, p.Surface.LakeLevel} {
			if len(a) != p.Surface.Width*p.Surface.Height {
				return nil, nil, fmt.Errorf("incomplete terrain surface")
			}
			if err := binary.Write(&b, binary.LittleEndian, a); err != nil {
				return nil, nil, err
			}
		}
	}
	return meta, b.Bytes(), nil
}
func Decode(meta, fields []byte) (*Planet, error) {
	var p Planet
	// Refined river vertices on the largest supported wet planets can exceed
	// the former 4 MiB envelope. Keep a bounded budget and validate every reach.
	if len(meta) > 64<<20 || json.Unmarshal(meta, &p) != nil {
		return nil, fmt.Errorf("invalid continuous terrain metadata")
	}
	if p.Schema != Schema {
		return nil, fmt.Errorf("unsupported continuous terrain schema")
	}
	if err := p.Spec.Validate(); err != nil {
		return nil, err
	}
	validGrid := p.Grid.Width == 192 && p.Grid.Height == 96
	if p.Spec.Version == Version {
		validGrid = p.Grid.Width == 512 && p.Grid.Height == 256
	}
	if !validGrid || len(p.Masses) > 1000 || len(p.Rivers) > 100000 || !Finite(p.SeaOffset) || p.SeaOffset < -.02 || p.SeaOffset > .02 {
		return nil, fmt.Errorf("invalid continuous terrain dimensions")
	}
	for _, m := range p.Masses {
		if !Finite(m.Center[0]) || !Finite(m.Center[1]) || m.Center[1] < -1.571 || m.Center[1] > 1.571 || !Finite(m.Rotation) || !Finite(m.Extent) || m.Extent < 0 || m.Extent > 4 || len(m.Strokes)+len(m.Cuts) > 64 {
			return nil, fmt.Errorf("invalid landform geometry")
		}
		for _, s := range append(append([]Stroke{}, m.Strokes...), m.Cuts...) {
			if len(s.Knots) < 1 || len(s.Knots) > 64 {
				return nil, fmt.Errorf("invalid landform stroke")
			}
			for _, k := range s.Knots {
				for _, v := range k {
					if !Finite(v) || v < -4 || v > 4 {
						return nil, fmt.Errorf("invalid landform coordinate")
					}
				}
				if k[2] < 0 {
					return nil, fmt.Errorf("negative landform radius")
				}
			}
		}
	}
	if len(p.Lakes) > p.Grid.Width*p.Grid.Height {
		return nil, fmt.Errorf("too many lake basins")
	}
	if len(p.Plates) > 64 {
		return nil, fmt.Errorf("too many tectonic plates")
	}
	for _, plate := range p.Plates {
		norm := 0.
		for k := 0; k < 3; k++ {
			if !Finite(plate.Center[k]) || !Finite(plate.Motion[k]) || math.Abs(plate.Motion[k]) > 4 {
				return nil, fmt.Errorf("invalid plate")
			}
			norm += plate.Center[k] * plate.Center[k]
		}
		if math.Abs(norm-1) > .0001 {
			return nil, fmt.Errorf("invalid spherical plate centre")
		}
	}
	for _, l := range p.Lakes {
		if !Finite(l.Center[0]) || !Finite(l.Center[1]) || math.Abs(l.Center[1]) > math.Pi/2 || !Finite(l.Radius) || l.Radius <= 0 || l.Radius > .2 || !Finite(l.Level) || l.Level < 0 || l.Level > 20000 {
			return nil, fmt.Errorf("invalid lake basin")
		}
		if len(l.Path) > 1000 || !Finite(l.Width) || l.Width < 0 || l.Width > 10000 || len(l.Path) > 0 && l.Width == 0 {
			return nil, fmt.Errorf("invalid lake channel")
		}
		for _, pt := range l.Path {
			if !Finite(pt[0]) || !Finite(pt[1]) || math.Abs(pt[1]) > math.Pi/2 {
				return nil, fmt.Errorf("invalid lake channel point")
			}
		}
	}
	r := bytes.NewReader(fields)
	magic := make([]byte, 4)
	io.ReadFull(r, magic)
	var count uint32
	binary.Read(r, binary.LittleEndian, &count)
	fieldLimit := uint32(64)
	if p.Spec.ClimateVersion >= 1 {
		fieldLimit += 7 // Persisted monthly extrema and seasonal rainfall summaries.
	}
	if string(magic) != "KTF1" || count < 14 || count > fieldLimit {
		return nil, fmt.Errorf("invalid terrain field header")
	}
	p.Grid.Fields = map[string][]float64{}
	n := p.Grid.Width * p.Grid.Height
	for i := uint32(0); i < count; i++ {
		var size uint16
		if binary.Read(r, binary.LittleEndian, &size) != nil || size == 0 || size > 80 {
			return nil, fmt.Errorf("invalid field name")
		}
		name := make([]byte, size)
		if _, err := io.ReadFull(r, name); err != nil {
			return nil, err
		}
		key := string(name)
		if _, exists := p.Grid.Fields[key]; exists {
			return nil, fmt.Errorf("duplicate terrain field")
		}
		a := make([]float64, n)
		if err := binary.Read(r, binary.LittleEndian, a); err != nil {
			return nil, err
		}
		for _, v := range a {
			if !Finite(v) {
				return nil, fmt.Errorf("non-finite terrain field")
			}
		}
		p.Grid.Fields[key] = a
	}
	if p.Spec.Version == Version && p.Surface == nil {
		return nil, fmt.Errorf("missing completed terrain surface")
	}
	if p.Surface != nil {
		s := p.Surface
		if (s.Width != 1024 && s.Width != 2048 && s.Width != 4096) || s.Height != s.Width/2 {
			return nil, fmt.Errorf("invalid surface dimensions")
		}
		if _, err := io.ReadFull(r, magic); err != nil || string(magic) != "KTS1" {
			return nil, fmt.Errorf("missing surface data")
		}
		for _, a := range []*[]float32{&s.Bed, &s.Coast, &s.Roughness, &s.LakeLevel} {
			*a = make([]float32, s.Width*s.Height)
			if err := binary.Read(r, binary.LittleEndian, *a); err != nil {
				return nil, err
			}
			for _, v := range *a {
				if !Finite(float64(v)) {
					return nil, fmt.Errorf("invalid surface sample")
				}
			}
		}
		vertices := 0
		for _, coast := range s.Coasts {
			for _, pt := range coast {
				vertices++
				if vertices > 2000000 || !Finite(pt[0]) || !Finite(pt[1]) || math.Abs(pt[1]) > math.Pi/2 {
					return nil, fmt.Errorf("invalid saved shoreline")
				}
			}
		}
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("extra terrain field data")
	}
	if p.Spec.Version != "continuous-1" {
		for _, key := range []string{"soil", "aridity", "rainShadow", "exposure", "seasonality", "dryMonths", "warmDry", "coldRain", "slope"} {
			if len(p.Grid.Fields[key]) != n {
				return nil, fmt.Errorf("missing climate field %s", key)
			}
		}
		if p.Spec.ClimateVersion >= 1 && p.Spec.Version == Version {
			for _, key := range []string{"coldest", "warmest", "rainDriest", "summerDriest", "winterWettest", "summerRain", "winterRain"} {
				if len(p.Grid.Fields[key]) != n {
					return nil, fmt.Errorf("missing seasonal field %s", key)
				}
			}
			for month := 0; month < 12; month++ {
				for _, prefix := range []string{"temperature", "rainfall", "pet"} {
					key := fmt.Sprintf("%s-%02d", prefix, month)
					if len(p.Grid.Fields[key]) != n {
						return nil, fmt.Errorf("missing monthly field %s", key)
					}
				}
			}
		}
	}
	for _, key := range []string{"bed", "coast", "roughness", "temperature", "precipitation", "pet", "moisture", "flow", "discharge", "drainage", "lake", "windX", "windY"} {
		if len(p.Grid.Fields[key]) != n {
			return nil, fmt.Errorf("missing terrain field %s", key)
		}
	}
	for i, v := range p.Grid.Fields["flow"] {
		j := int(v)
		if v != float64(j) || j < -1 || j >= n || j == i {
			return nil, fmt.Errorf("invalid drainage edge")
		}
		if j >= 0 && p.Grid.Fields["drainage"][j] >= p.Grid.Fields["drainage"][i] {
			return nil, fmt.Errorf("non-descending drainage edge")
		}
	}
	for _, river := range p.Rivers {
		n := len(river.Path)
		if n < 2 || n > 20000 || len(river.Bed) != n || len(river.Surface) != n || len(river.Discharge) != n || len(river.Width) != n {
			return nil, fmt.Errorf("invalid river geometry")
		}
		for i, pt := range river.Path {
			if !Finite(pt[0]) || !Finite(pt[1]) || pt[1] < -1.571 || pt[1] > 1.571 || !Finite(river.Bed[i]) || !Finite(river.Surface[i]) || !Finite(river.Discharge[i]) || !Finite(river.Width[i]) || river.Width[i] < 0 || river.Discharge[i] < 0 || river.Bed[i] > river.Surface[i] {
				return nil, fmt.Errorf("invalid river data")
			}
			if i > 0 && (river.Surface[i] > river.Surface[i-1] || river.Discharge[i] < river.Discharge[i-1]) {
				return nil, fmt.Errorf("invalid river direction")
			}
		}
	}
	return &p, nil
}
