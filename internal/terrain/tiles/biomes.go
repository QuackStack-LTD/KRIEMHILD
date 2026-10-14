package tiles

// Shared by the categorical renderer and its API legend. Unknown IDs are magenta.
var BiomeColors = map[string][3]uint8{
	"water": {45, 110, 147}, "desert": {221, 188, 121}, "steppe": {181, 162, 103},
	"grassland": {157, 184, 100}, "savanna": {189, 179, 81}, "shrubland": {166, 151, 109},
	"woodland": {112, 151, 91}, "tropical-woodland": {89, 144, 65}, "forest": {55, 119, 76},
	"rainforest": {30, 94, 65}, "temperate-rainforest": {48, 128, 113}, "taiga": {85, 125, 116},
	"alpine": {151, 148, 139}, "tundra": {166, 170, 148}, "wetland": {84, 146, 145}, "ice": {232, 241, 239},
}
