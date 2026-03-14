import cytoscape from 'cytoscape';
import { DOMAIN_COLORS } from './domain-colors';

// Generate domain overlay styles for broadcast (bd) and collision (cd) domains
function generateDomainStyles(): cytoscape.StylesheetStyle[] {
  const styles: cytoscape.StylesheetStyle[] = [];
  for (let i = 0; i < DOMAIN_COLORS.length; i++) {
    const color = DOMAIN_COLORS[i];
    // Broadcast domain - nodes: solid border + semi-transparent background
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
    // Broadcast domain - edges: colored line
    styles.push({
      selector: `edge.domain-bd-${i}`,
      style: {
        'line-color': color,
        'width': 4,
      }
    });
    // Collision domain - nodes: dashed border
    styles.push({
      selector: `node.domain-cd-${i}`,
      style: {
        'border-width': 3,
        'border-color': color,
        'border-opacity': 1,
        'border-style': 'dashed',
      }
    });
    // Collision domain - edges: dashed colored line
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

export const CYTOSCAPE_STYLES: cytoscape.StylesheetStyle[] = [
  {
    selector: 'node',
    style: {
      'label': 'data(label)',
      'color': '#1e293b',
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
    selector: 'node[type="hub"]',
    style: {
      'background-image': 'assets/icons/hub.svg',
      'background-fit': 'contain',
      'background-clip': 'none',
      'width': 52,
      'height': 52
    }
  },
  {
    selector: 'node[type="cloud"]',
    style: {
      'background-image': 'assets/icons/cloud.svg',
      'background-fit': 'contain',
      'background-clip': 'none',
      'width': 52,
      'height': 52
    }
  },
  {
    selector: 'node[type="linux"]',
    style: {
      'background-image': 'assets/icons/linux.svg',
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
      'font-size': '10px',
      'font-family': 'JetBrains Mono, monospace',
      'color': '#475569',
      'text-wrap': 'wrap',
      'text-background-opacity': 1,
      'text-background-color': '#f1f5f9',
      'text-background-padding': '3px',
      'text-background-shape': 'roundrectangle'
    }
  },
  // Disabled link style
  {
    selector: 'edge.link-disabled',
    style: {
      'line-color': '#475569',
      'line-style': 'dashed',
      'opacity': 0.4,
    }
  },
  // Traceroute path highlight
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
  ...generateDomainStyles()
];
