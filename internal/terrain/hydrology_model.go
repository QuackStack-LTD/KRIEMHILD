package terrain

import "fmt"

// Flow units are 1,000 mm of water over one base-grid cell per year. These
// annual/seasonal budgets are scale-independent proxies, not SI river m³/s.
var HydrologyFields = []string{"permeability", "lithology", "runoff", "recharge", "baseflow", "evapotranspiration", "snowmelt", "glacierMelt", "dryRunoff", "dryDischarge", "groundFlow", "waterTableDepth", "streamOrder", "watershed", "hydroLoss", "terminalBasin", "beach", "coastalSediment", "coastType", "spring", "geothermal", "river", "freshwaterDistance"}

type HydroDiagnostic struct {
	Object   string `json:"object"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
}
type WaterBudget struct {
	Inflow       float64 `json:"inflow"`
	Rain         float64 `json:"rain"`
	Evaporation  float64 `json:"evaporation"`
	Infiltration float64 `json:"infiltration"`
	Outflow      float64 `json:"outflow"`
}
type HydroBasin struct {
	WaterBodies []int       `json:"waterBodies"`
	Outlets     []int       `json:"outlets"`
	ID          string      `json:"id"`
	Cells       []int       `json:"cells"`
	Floor       float64     `json:"floor"`
	Spill       float64     `json:"spill"`
	Level       float64     `json:"level"`
	Origin      string      `json:"origin"`
	Regime      string      `json:"regime"`
	WaterBody   int         `json:"waterBody"`
	Outlet      int         `json:"outlet"`
	Budget      WaterBudget `json:"budget"`
	Salinity    float64     `json:"salinity"`
	Inlets      []string    `json:"inlets"`
	Downstream  string      `json:"downstream"`
}
type HydroReach struct {
	DischargeM3s float64       `json:"dischargeM3s,omitempty"`
	WidthMetres  float64       `json:"widthMetres,omitempty"`
	RiverID      string        `json:"riverId,omitempty"`
	Class        string        `json:"class,omitempty"`
	Branches     []HydroBranch `json:"branches,omitempty"`
	ID           string        `json:"id"`
	From         int           `json:"from"`
	To           int           `json:"to"`
	Upstream     []string      `json:"upstream"`
	Downstream   string        `json:"downstream"`
	Watershed    string        `json:"watershed"`
	Source       string        `json:"source"`
	Regime       string        `json:"regime"`
	Morphology   string        `json:"morphology"`
	Order        int           `json:"order"`
	Discharge    float64       `json:"discharge"`
	DryDischarge float64       `json:"dryDischarge"`
	Width        float64       `json:"width"`
}
type HydroBranch struct {
	ID         string  `json:"id"`
	To         int     `json:"to"`
	Downstream string  `json:"downstream"`
	Fraction   float64 `json:"fraction"`
}
type MarineRegion struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Cells       []int    `json:"cells"`
	Connections []string `json:"connections"`
}
type HydroSpring struct {
	ID          string             `json:"id"`
	Cell        int                `json:"cell"`
	Origin      string             `json:"origin"`
	Downstream  string             `json:"downstream"`
	Discharge   float64            `json:"discharge"`
	Temperature float64            `json:"temperature"`
	Minerals    map[string]float64 `json:"minerals"`
}
type HydroWetland struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Cells    []int  `json:"cells"`
	Source   string `json:"source"`
	Seasonal bool   `json:"seasonal"`
}
type HydroCoast struct {
	ID        string `json:"id"`
	Cell      int    `json:"cell"`
	Kind      string `json:"kind"`
	WaterBody int    `json:"waterBody"`
	River     string `json:"river,omitempty"`
}
type HydroCanal struct {
	ID          string  `json:"id"`
	Purpose     string  `json:"purpose"`
	Source      string  `json:"source"`
	Destination string  `json:"destination"`
	Path        []int   `json:"path"`
	Supply      float64 `json:"supply"`
	Locks       []int   `json:"locks"`
	Pumps       []int   `json:"pumps"`
	Aqueducts   []int   `json:"aqueducts"`
}
type HydrologyState struct {
	NetworkVersion int               `json:"networkVersion,omitempty"`
	Rivers         []RiverSystem     `json:"rivers,omitempty"`
	Watersheds     []DrainageBasin   `json:"watersheds,omitempty"`
	Targets        []DrainageTarget  `json:"targets,omitempty"`
	Statistics     *HydroStatistics  `json:"statistics,omitempty"`
	MarineRegions  []MarineRegion    `json:"marineRegions"`
	Version        int               `json:"version"`
	Units          string            `json:"units"`
	Basins         []HydroBasin      `json:"basins"`
	Reaches        []HydroReach      `json:"reaches"`
	Springs        []HydroSpring     `json:"springs"`
	Wetlands       []HydroWetland    `json:"wetlands"`
	Coasts         []HydroCoast      `json:"coasts"`
	Canals         []HydroCanal      `json:"canals"`
	Diagnostics    []HydroDiagnostic `json:"diagnostics"`
}

func (e *Environment) hydroID(kind string, cell int) string {
	return fmt.Sprintf("%x/%s/%d", Seed(e.Options.Seed), kind, cell)
}
func (e *Environment) riverID(cell int) string {
	return fmt.Sprintf("%x/drainage/%d", Seed(e.Options.Seed)^0x91ab6731, cell)
}
