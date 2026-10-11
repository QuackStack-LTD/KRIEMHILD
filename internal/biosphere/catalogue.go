package biosphere

// Reference niches are deliberately broad game envelopes, not empirical species
// survival limits. Catalogue packs share this schema and are validated on import.
func Catalogue() []Species {
	land := Requirements{Medium: "terrestrial", Temperature: Range{-30, 40}, Moisture: Range{.2, 1}, Elevation: Range{-500, 4000}, Depth: Range{0, 0}, Salinity: Range{0, 5}, MinLight: .05}
	water := Requirements{Medium: "freshwater", Temperature: Range{-10, 38}, Moisture: Range{0, 1}, Elevation: Range{-500, 5000}, Depth: Range{0, 5000}, Salinity: Range{0, 5}}
	marine := water
	marine.Medium = "marine"
	marine.Elevation.Min = -12000
	marine.Salinity = Range{5, 50}
	marine.Depth.Max = 12000
	all := []Species{}
	add := func(id, name, group, role string, r Requirements, habitats []string, deps ...Dependency) {
		r.Habitats = habitats
		all = append(all, Species{ID: id, Name: name, Summary: "Representative ecological reference; tolerance values are procedural game approximations.", Biology: Lifeform{Version: 1, Group: group, Origin: "earth-reference", Roles: []string{role}, Traits: map[string]string{}, Dispersal: "regional", Requirements: r, Dependencies: deps, Completeness: "draft", CatalogueID: "core-v1/" + id}})
	}
	feed := func(role string) Dependency { return Dependency{Role: role, Kind: "feeds-on"} }
	add("broadleaf", "Temperate broadleaf tree", "plant", "producer", land, []string{"temperate-forest", "floodplain"})
	cold := land
	cold.Temperature = Range{-55, 28}
	cold.Elevation.Max = 6000
	cold.Moisture.Min = .05
	add("evergreen", "Cold-adapted evergreen shrub", "plant", "producer", cold, []string{"boreal-forest", "tundra", "alpine"})
	tropical := land
	tropical.Temperature = Range{10, 48}
	tropical.Moisture.Min = .4
	add("tropical-tree", "Tropical canopy tree", "plant", "producer", tropical, []string{"tropical-forest"})
	dry := land
	dry.Temperature = Range{-15, 55}
	dry.Moisture = Range{0, .6}
	dry.MinLight = .2
	add("succulent", "Desert succulent", "plant", "producer", dry, []string{"desert", "desert-dunes", "oasis"})
	add("grass", "Meadow grasses", "plant", "producer", land, []string{"grassland", "floodplain", "alpine"})
	shade := cold
	shade.MinLight = .02
	shade.Moisture.Min = .3
	add("moss", "Forest-floor moss", "plant", "producer", shade, []string{"temperate-forest", "boreal-forest", "wetland", "tundra"})
	wet := land
	wet.Medium = "amphibious"
	wet.Depth.Max = 5
	wet.Moisture.Min = .4
	add("reed", "Emergent wetland reed", "plant", "producer", wet, []string{"wetland", "lake", "river", "floodplain"})
	algae := water
	algae.MinLight = .08
	add("fresh-algae", "Freshwater algae", "algae", "producer", algae, []string{"river", "lake", "wetland"})
	plankton := marine
	plankton.Depth.Max = 12000
	plankton.MinLight = 0
	// Ocean cells describe seabed depth; pelagic producers occupy the photic surface.
	add("phytoplankton", "Marine phytoplankton", "algae", "producer", plankton, nil)
	all[len(all)-1].Biology.Traits["verticalRange"] = "Photic surface above the seabed"
	decomposer := land
	decomposer.Moisture.Min = .15
	decomposer.MinLight = 0
	decomposer.Temperature = Range{-40, 50}
	add("saprotroph", "Saprotrophic soil fungus", "fungus", "decomposer", decomposer, nil, Dependency{Role: "producer", Kind: "decomposes"})
	broad := land
	broad.Medium = "any"
	broad.Temperature = Range{-70, 70}
	broad.Moisture = Range{0, 1}
	broad.Elevation = Range{-12000, 8500}
	broad.Depth.Max = 12000
	broad.Salinity.Max = 55
	broad.MinLight = 0
	add("bacteria", "Decomposer bacterial community", "bacterium", "decomposer", broad, nil, Dependency{Role: "producer", Kind: "decomposes"})
	add("grazer", "Small terrestrial grazer", "animal", "herbivore", land, nil, feed("producer"))
	add("desert-grazer", "Desert-adapted grazer", "animal", "herbivore", dry, []string{"desert", "oasis", "desert-dunes"}, feed("producer"))
	add("cold-grazer", "Cold-adapted alpine grazer", "animal", "herbivore", cold, []string{"alpine", "tundra", "boreal-forest"}, feed("producer"))
	add("fresh-grazer", "Freshwater grazing invertebrate", "animal", "herbivore", water, nil, feed("producer"))
	add("zooplankton", "Marine grazing zooplankton", "animal", "herbivore", marine, nil, feed("producer"))
	add("red-fox", "Red fox", "animal", "predator", land, []string{"temperate-forest", "boreal-forest", "grassland", "floodplain"}, feed("herbivore"))
	all[len(all)-1].Biology.ScientificName = "Vulpes vulpes"
	all[len(all)-1].Biology.Traits["reference"] = "https://www.nps.gov/yell/learn/nature/red-fox.htm"
	add("fresh-fish", "Freshwater predatory fish", "animal", "predator", water, nil, feed("herbivore"))
	add("marine-fish", "Marine predatory fish", "animal", "predator", marine, nil, feed("herbivore"))
	add("pollinator", "Flower-visiting insect", "animal", "pollinator", land, nil, Dependency{Group: "plant", Role: "producer", Kind: "pollinates"})
	add("protist", "Bacteria-grazing protist", "protist", "omnivore", water, nil, Dependency{Group: "bacterium", Kind: "feeds-on"})
	geothermal := broad
	geothermal.Substrates = []string{"geothermal"}
	add("archaea", "Geothermal archaeal community", "archaea", "producer", geothermal, []string{"geothermal"})
	all[len(all)-1].Biology.Traits["energy"] = "Chemical energy from geothermal substrate"
	all[len(all)-1].Biology.Traits["reference"] = "https://www.nps.gov/yell/learn/nature/thermophilic-archaea.htm"
	add("phage", "Host-associated bacteriophage", "virus", "host-dependent", broad, nil, Dependency{Group: "bacterium", Kind: "infects", Description: "Compatible bacterial host required; infection does not imply disease"})
	all[len(all)-1].Biology.Traits["transmission"] = "Local contact with compatible bacterial hosts in soil or water"
	return all
}
