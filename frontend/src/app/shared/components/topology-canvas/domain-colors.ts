/** 10 distinct colors for domain visualization. */
export const DOMAIN_COLORS = [
  '#3b82f6', // blue
  '#ef4444', // red
  '#22c55e', // green
  '#f59e0b', // amber
  '#a855f7', // purple
  '#06b6d4', // cyan
  '#f97316', // orange
  '#ec4899', // pink
  '#14b8a6', // teal
  '#8b5cf6', // violet
];

export function getDomainColor(index: number): string {
  return DOMAIN_COLORS[index % DOMAIN_COLORS.length];
}
