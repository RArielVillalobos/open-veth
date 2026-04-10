import cytoscape from 'cytoscape';
import { DOMAIN_COLORS, ROUTE_HIGHLIGHT_COLOR, ROUTE_HIGHLIGHT_STATIC_COLOR } from './domain-colors';

function generateDomainStyles(): cytoscape.StylesheetStyle[] {
  const styles: cytoscape.StylesheetStyle[] = [];
  for (let i = 0; i < DOMAIN_COLORS.length; i++) {
    const color = DOMAIN_COLORS[i];
    styles.push({
      selector: `node.domain-bd-${i}`,
      style: {
        'border-width': 3,
        'border-color': color,
        'border-opacity': 1,
        'background-color': color,
        'background-opacity': 0.15,
      }
    });
    styles.push({
      selector: `edge.domain-bd-${i}`,
      style: {
        'line-color': color,
        'width': 4,
      }
    });
    styles.push({
      selector: `node.domain-cd-${i}`,
      style: {
        'border-width': 3,
        'border-color': color,
        'border-opacity': 1,
        'border-style': 'dashed',
      }
    });
    styles.push({
      selector: `edge.domain-cd-${i}`,
      style: {
        'line-color': color,
        'line-style': 'dashed',
        'width': 4,
      }
    });
  }
  return styles;
}

function nodeImageStyle(url: string): cytoscape.StylesheetStyle['style'] {
  return {
    'background-image': url,
    'background-fit': 'contain',
    'background-clip': 'none',
    'width': 52,
    'height': 52,
  };
}

export function getCytoscapeStyles(bg: Record<string, string>): cytoscape.StylesheetStyle[] {
  const icon = (name: string) => bg[name] || `assets/icons/${name}.svg`;
  return [
    {
      selector: 'node',
      style: {
        'label': 'data(label)',
        'color': '#e2e8f0',
        'font-size': '12px',
        'font-weight': 'bold',
        'text-valign': 'bottom',
        'text-wrap': 'wrap',
        'text-max-width': '150px',
        'text-margin-y': -2,
        'width': 52,
        'height': 52,
        'shape': 'rectangle',
        'background-color': 'transparent',
        'background-opacity': 0,
        'border-width': 0,
        'text-outline-color': '#020617',
        'text-outline-width': 2
      }
    },
    { selector: 'node[type="router"]',  style: nodeImageStyle(icon('router'))  },
    { selector: 'node[type="switch"]',  style: nodeImageStyle(icon('switch'))  },
    { selector: 'node[type="host"]',    style: nodeImageStyle(icon('host'))    },
    { selector: 'node[type="hub"]',     style: nodeImageStyle(icon('hub'))     },
    { selector: 'node[type="cloud"]',   style: nodeImageStyle(icon('cloud'))   },
    { selector: 'node[type="server"]',  style: nodeImageStyle(icon('server'))  },
    { selector: 'node[type="monitor"]', style: nodeImageStyle(icon('monitor')) },
    { selector: 'node[type="tester"]',  style: nodeImageStyle(icon('tester'))  },
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
        'label': 'data(subnet)',
        'text-rotation': 'autorotate',
        'source-label': 'data(source_int)',
        'target-label': 'data(target_int)',
        'source-text-offset': 32,
        'target-text-offset': 32,
        'font-size': '10px',
        'font-family': 'JetBrains Mono, monospace',
        'color': '#cbd5e1',
        'text-wrap': 'wrap',
        'text-background-opacity': 0.85,
        'text-background-color': '#0f172a',
        'text-background-padding': '3px',
        'text-background-shape': 'roundrectangle'
      }
    },
    {
      selector: 'edge.link-disabled',
      style: {
        'line-color': '#475569',
        'line-style': 'dashed',
        'opacity': 0.4,
      }
    },
    {
      selector: 'node.node-stopped',
      style: {
        'opacity': 0.4,
      }
    },
    {
      selector: 'node.traceroute-path',
      style: {
        'border-width': 3,
        'border-color': '#22c55e',
        'border-opacity': 1,
        'background-color': '#22c55e',
        'background-opacity': 0.15,
      }
    },
    {
      selector: 'edge.traceroute-path',
      style: {
        'line-color': '#22c55e',
        'width': 4,
      }
    },
    ...generateDomainStyles(),
  ...generateSubnetStyles(),
    {
      selector: 'edge.route-highlight',
      style: {
        'line-color': ROUTE_HIGHLIGHT_COLOR,
        'width': 6,
        'opacity': 1,
      }
    },
    {
      selector: 'edge.route-highlight-static',
      style: {
        'line-color': ROUTE_HIGHLIGHT_STATIC_COLOR,
        'width': 6,
        'opacity': 1,
      }
    },
  ];
}

function generateSubnetStyles(): cytoscape.StylesheetStyle[] {
  return DOMAIN_COLORS.map((color, i) => ({
    selector: `edge.subnet-${i}`,
    style: {
      'line-color': color,
      'width': 4,
    }
  }));
}
