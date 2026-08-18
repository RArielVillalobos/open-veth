/**
 * "Signal Console" chrome tokens — mirrors the slate/signal/up/pending/down
 * scale in tailwind.config.js.
 *
 * Cytoscape stylesheets and <canvas> 2D drawing can't consume Tailwind
 * classes, so this is the canvas layer's copy of the same values, kept in
 * one place instead of hardcoded hex scattered across cytoscape-styles.ts
 * and topology-canvas.component.ts. Categorical colors (per-node-type
 * icons, DOMAIN_COLORS, route highlight) stay out of this file on purpose —
 * they're meant to be distinct hues, not brand chrome.
 */
export const GROUND = '#0d1416';
export const SURFACE = '#131b1e';
export const SURFACE_RAISED = '#1a2427';
export const LINE = '#28353a';

export const INK = '#e8efef';
export const INK_DIM = '#8fa3a6';
export const INK_FAINT = '#5c6d70';

export const SIGNAL = '#4fb3ac';
export const SIGNAL_HI = '#7ed6ce';

export const UP = '#8fbb6c';
export const UP_600 = '#6f9c4f';
export const PENDING = '#d7a153';
export const DOWN = '#d97b68';
