const SVG_NS = 'http://www.w3.org/2000/svg';
const MARKER_ID = 'goflow-workflow-arrow';
const DEFINITIONS_ID = 'goflow-workflow-arrow-definitions';

function svgElement(name, attributes = {}) {
  const element = document.createElementNS(SVG_NS, name);
  Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, String(value)));
  return element;
}

export function installWorkflowArrowMarkers() {
  if (typeof document === 'undefined' || document.getElementById(DEFINITIONS_ID)) return;

  const svg = svgElement('svg', {
    id: DEFINITIONS_ID,
    width: 0,
    height: 0,
    'aria-hidden': 'true',
    focusable: 'false',
  });
  svg.style.position = 'absolute';
  svg.style.width = '0';
  svg.style.height = '0';
  svg.style.overflow = 'hidden';
  svg.style.pointerEvents = 'none';

  const defs = svgElement('defs');
  const marker = svgElement('marker', {
    id: MARKER_ID,
    viewBox: '0 0 18 18',
    markerWidth: 18,
    markerHeight: 18,
    refX: 18,
    refY: 9,
    orient: 'auto',
    markerUnits: 'userSpaceOnUse',
  });
  const arrow = svgElement('path', {
    d: 'M 1 1 L 18 9 L 1 17 Z',
    fill: '#1d1c12',
    stroke: '#1d1c12',
    'stroke-width': 1,
    'stroke-linejoin': 'miter',
  });

  marker.appendChild(arrow);
  defs.appendChild(marker);
  svg.appendChild(defs);
  document.body.prepend(svg);
}
