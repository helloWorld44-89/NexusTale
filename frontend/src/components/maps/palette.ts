// Icon palettes for the Map Studio canvas — which set shows depends on the
// map's scale (map_type). Palette is just data; adding an icon later is a
// one-line addition here plus a case in MapIcons.tsx, no schema change.

export type PaletteItem = { icon_type: string; label: string }

export const TERRAIN_PALETTE: PaletteItem[] = [
  { icon_type: 'mountain', label: 'Mountain' },
  { icon_type: 'lake', label: 'Lake' },
  { icon_type: 'river', label: 'River' },
  { icon_type: 'ocean', label: 'Ocean' },
  { icon_type: 'forest', label: 'Forest' },
  { icon_type: 'desert', label: 'Desert' },
  { icon_type: 'swamp', label: 'Swamp' },
  { icon_type: 'city', label: 'City' },
]

export const SPACE_PALETTE: PaletteItem[] = [
  { icon_type: 'star', label: 'Star' },
  { icon_type: 'planet', label: 'Planet' },
  { icon_type: 'moon', label: 'Moon' },
  { icon_type: 'nebula', label: 'Nebula' },
  { icon_type: 'asteroid_field', label: 'Asteroid Field' },
  { icon_type: 'space_station', label: 'Space Station' },
  { icon_type: 'wormhole', label: 'Wormhole' },
  { icon_type: 'black_hole', label: 'Black Hole' },
]

export const MAP_TYPES = ['world', 'region', 'city', 'galaxy', 'planet', 'custom'] as const
export type MapType = typeof MAP_TYPES[number]

export function paletteForMapType(mapType: string): PaletteItem[] {
  return mapType === 'galaxy' || mapType === 'planet' ? SPACE_PALETTE : TERRAIN_PALETTE
}

// Region fill colors offered when closing a drawn shape — a small curated
// set rather than a free color picker, keeps map styling consistent.
export const REGION_COLORS: { type: string; label: string; color: string }[] = [
  { type: 'landmass', label: 'Landmass', color: '#4a7c3a' },
  { type: 'ocean', label: 'Ocean', color: '#2a6f8f' },
  { type: 'forest', label: 'Forest', color: '#2d5a2d' },
  { type: 'desert', label: 'Desert', color: '#c2a24a' },
  { type: 'tundra', label: 'Tundra', color: '#a9c2c9' },
  { type: 'nebula_cloud', label: 'Nebula Cloud', color: '#7a4fa3' },
  { type: 'gravity_well', label: 'Gravity Well', color: '#2b2b3d' },
]
