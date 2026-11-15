const versionFourNames = {
  forest: "Temperate broadleaf and mixed forest",
  woodland: "Temperate woodland",
  grassland: "Temperate grassland",
  "tropical-woodland": "Tropical seasonal forest and woodland",
  shrubland: "Mediterranean woodland and scrub",
  taiga: "Boreal forest (taiga)",
  "temperate-steppe": "Temperate steppe",
  "subtropical-steppe": "Subtropical steppe",
  "tropical-steppe": "Tropical steppe",
};

export function biomeName(name, climateVersion) {
  return climateVersion >= 4 ? versionFourNames[name] || name : name;
}
