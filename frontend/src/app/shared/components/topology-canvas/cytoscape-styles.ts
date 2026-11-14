import cytoscape from 'cytoscape';
import { DOMAIN_COLORS, ROUTE_HIGHLIGHT_COLOR, ROUTE_HIGHLIGHT_STATIC_COLOR } from './domain-colors';
import { GROUND, SURFACE, INK, INK_FAINT, UP } from './theme-colors';

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
    'background-image': (node: cytoscape.NodeSingular) => {
      const colors: Record<string, string> = { running: '#8eaf9f', stopped: '#8fa3a6', pending: '#c3ac81', error: '#e2917f' };
      const color = colors[node.data('status')] ?? '#8fa3a6';
      const badge = '<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12"><circle cx="6" cy="6" r="4.5" fill="' + color + '" stroke="#0d1416" stroke-width="2"/></svg>';
      return [url, 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(badge)];
    },
    'background-fit': ['contain', 'none'],
    'background-width': ['100%', 12],
    'background-height': ['100%', 12],
    'background-position-x': ['50%', '100%'],
    'background-position-y': ['50%', '100%'],
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
        'color': INK,
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
        'text-outline-color': GROUND,
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
    { selector: 'node[type="haproxy"]', style: nodeImageStyle(icon('haproxy')) },
    { selector: 'node[type="storage"]', style: nodeImageStyle(icon('storage')) },
    { selector: 'node[type="zabbix"]',  style: nodeImageStyle(icon('zabbix'))  },
    {
      selector: '.terminal-active',
      style: {
        'border-width': 2,
        'border-color': UP,
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
        'width': 4,
        'line-color': INK_FAINT,
        'curve-style': 'bezier',
        'label': '',
        'text-rotation': 'autorotate',
        'source-label': '',
        'target-label': '',
        'source-text-offset': 55,
        'target-text-offset': 55,
        'source-text-margin-y': -24,
        'target-text-margin-y': 24,
        'text-margin-y': 4,
        'font-size': '13px',
        'font-family': '"IBM Plex Mono", monospace',
        'color': INK,
        'text-wrap': 'wrap',
        'text-background-opacity': 0.9,
        'text-background-color': SURFACE,
        'text-background-padding': '5px',
        'text-background-shape': 'roundrectangle'
      }
    },
    {
      selector: 'edge.link-disabled',
      style: {
        'line-color': INK_FAINT,
        'line-style': 'dashed',
        'opacity': 0.4,
      }
    },
    {
      selector: 'node.node-stopped',
      style: {
        'opacity': 1,
      }
    },
    {
      selector: 'node.traceroute-path',
      style: {
        'border-width': 3,
        'border-color': UP,
        'border-opacity': 1,
        'background-color': UP,
        'background-opacity': 0.15,
      }
    },
    {
      selector: 'edge.traceroute-path',
      style: {
        'line-color': UP,
        'width': 4,
      }
    },
    ...generateDomainStyles(),

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
