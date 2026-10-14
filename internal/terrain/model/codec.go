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
	return meta, b.Bytes(), nil
}
func Decode(meta, fields []byte) (*Planet, error) {
	var p Planet
	// Refined river vertices on the largest supported wet planets can exceed
	// the former 4 MiB envelope. Keep a bounded budget and validate every reach.
	if len(meta) > 16<<20 || json.Unmarshal(meta, &p) != nil {
		return nil, fmt.Errorf("invalid continuous terrain metadata")
	}
	if p.Schema != Schema {
		return nil, fmt.Errorf("unsupported continuous terrain schema")
	}
	if err := p.Spec.Validate(); err != nil {
		return nil, err
	}
	if p.Grid.Width != 192 || p.Grid.Height != 96 || len(p.Masses) > 1000 || len(p.Rivers) > 20000 || !Finite(p.SeaOffset) || p.SeaOffset < -.02 || p.SeaOffset > .02 {
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
	}
	r := bytes.NewReader(fields)
	magic := make([]byte, 4)
	io.ReadFull(r, magic)
	var count uint32
	binary.Read(r, binary.LittleEndian, &count)
	if string(magic) != "KTF1" || count < 14 || count > 64 {
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
	if r.Len() != 0 {
		return nil, fmt.Errorf("extra terrain field data")
	}
	if p.Spec.Version != "continuous-1" {
		for _, key := range []string{"soil", "aridity", "rainShadow", "exposure", "seasonality", "dryMonths", "warmDry", "coldRain", "slope"} {
			if len(p.Grid.Fields[key]) != n {
				return nil, fmt.Errorf("missing climate field %s", key)
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
