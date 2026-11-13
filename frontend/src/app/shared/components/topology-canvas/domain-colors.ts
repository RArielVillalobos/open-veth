/**
 * 10 distinct colors for domain/subnet visualization.
 * RESERVED — do not add: ROUTE_HIGHLIGHT_COLOR or ROUTE_HIGHLIGHT_STATIC_COLOR.
 */
export const ROUTE_HIGHLIGHT_COLOR = '#c7ad75';
export const ROUTE_HIGHLIGHT_STATIC_COLOR = '#c5cfd6';

export const DOMAIN_COLORS = [
  '#719bc4', // blue
  '#c47f79', // red
  '#83aa8c', // green
  '#b99660', // amber
  '#a08abd', // purple
  '#73a9b6', // cyan
  '#c28e71', // orange
  '#ba819f', // pink
  '#71a89d', // teal
  '#858bb8', // violet
];

export function getDomainColor(index: number): string {
  return DOMAIN_COLORS[index % DOMAIN_COLORS.length];
}
