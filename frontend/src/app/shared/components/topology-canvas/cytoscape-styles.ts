import cytoscape from 'cytoscape';

export const CYTOSCAPE_STYLES: cytoscape.StylesheetStyle[] = [
  {
    selector: 'node',
    style: {
      'label': 'data(label)',
      'color': '#1e293b',
      'font-size': '11px',
      'font-weight': 'bold',
      'text-valign': 'bottom',
      'text-wrap': 'wrap',
      'text-max-width': '150px',
      'text-margin-y': 6,
      'width': 56,
      'height': 48,
      'shape': 'rectangle',
      'background-color': 'transparent',
      'background-opacity': 0,
      'border-width': 0,
      'text-outline-color': '#f8fafc',
      'text-outline-width': 0
    }
  },
  {
    selector: 'node[type="router"]',
    style: {
      'background-image': 'assets/icons/router.svg',
      'background-fit': 'contain',
      'background-clip': 'none',
      'width': 52,
      'height': 52
    }
  },
  {
    selector: 'node[type="switch"]',
    style: {
      'background-image': 'assets/icons/switch.svg',
      'background-fit': 'contain',
      'background-clip': 'none',
      'width': 52,
      'height': 52
    }
  },
  {
    selector: 'node[type="host"]',
    style: {
      'background-image': 'assets/icons/host.svg',
      'background-fit': 'contain',
      'background-clip': 'none',
      'width': 52,
      'height': 52
    }
  },
  {
    selector: '.terminal-active',
    style: {
      'border-width': 2,
      'border-color': '#34d399',
      'border-opacity': 0.9,
    }
  },
  {
    selector: '.selected-source',
    style: {
      'border-width': 3,
      'border-color': '#8b5cf6',
      'border-style': 'solid',
      'background-color': '#ede9fe',
      'background-opacity': 0.6
    }
  },
  {
    selector: 'edge',
    style: {
      'width': 3,
      'line-color': '#94a3b8',
      'curve-style': 'bezier',
      'source-label': 'data(source_int)',
      'target-label': 'data(target_int)',
      'source-text-offset': 32,
      'target-text-offset': 32,
      'font-size': '9px',
      'color': '#6366f1',
      'text-wrap': 'wrap',
      'text-background-opacity': 1,
      'text-background-color': '#eef2ff',
      'text-background-padding': '2px',
      'text-background-shape': 'roundrectangle'
    }
  }
];
