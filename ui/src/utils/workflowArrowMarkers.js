import { useVueFlow } from '@vue-flow/core';

const SVG_NS = 'http://www.w3.org/2000/svg';
const MARKER_ID = 'goflow-workflow-arrow';
const DEFINITIONS_ID = 'goflow-workflow-arrow-definitions';
const HANDLE_POSITIONS = ['top', 'right', 'bottom', 'left'];

function svgElement(name, attributes = {}) {
  const element = document.createElementNS(SVG_NS, name);
  Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, String(value)));
  return element;
}

function installArrowDefinition() {
  if (document.getElementById(DEFINITIONS_ID)) return;

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
    viewBox: '0 0 18 14',
    markerWidth: 18,
    markerHeight: 14,
    refX: 18,
    refY: 7,
    orient: 'auto',
    markerUnits: 'userSpaceOnUse',
  });
  const arrow = svgElement('path', {
    d: 'M 2 1 L 13 7 L 2 13 Z',
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

function flowIdForHandle(handle, type) {
  const dataId = handle.getAttribute('data-id') || '';
  const nodeId = handle.getAttribute('data-nodeid') || '';
  const handleId = handle.getAttribute('data-handleid') ?? 'null';
  const suffix = `-${nodeId}-${handleId}-${type}`;
  return dataId.endsWith(suffix) ? dataId.slice(0, -suffix.length) : '';
}

function orientHandle(handle) {
  const type = handle.classList.contains('target')
    ? 'target'
    : handle.classList.contains('source')
      ? 'source'
      : '';
  if (!type) return null;

  const position = type === 'target' ? 'left' : 'right';
  const nodeId = handle.getAttribute('data-nodeid') || '';
  const flowId = flowIdForHandle(handle, type);
  const changed = handle.getAttribute('data-handlepos') !== position || !handle.classList.contains(`vue-flow__handle-${position}`);

  HANDLE_POSITIONS.forEach((candidate) => {
    if (candidate !== position) handle.classList.remove(`vue-flow__handle-${candidate}`);
  });
  handle.classList.add(`vue-flow__handle-${position}`);
  handle.setAttribute('data-handlepos', position);

  return changed && flowId && nodeId ? { flowId, nodeId } : null;
}

function refreshHandleGeometry() {
  const changedNodesByFlow = new Map();
  document.querySelectorAll('.goflow-canvas .vue-flow__handle').forEach((handle) => {
    const changed = orientHandle(handle);
    if (!changed) return;
    if (!changedNodesByFlow.has(changed.flowId)) changedNodesByFlow.set(changed.flowId, new Set());
    changedNodesByFlow.get(changed.flowId).add(changed.nodeId);
  });

  changedNodesByFlow.forEach((nodeIds, flowId) => {
    requestAnimationFrame(() => {
      const flow = useVueFlow(flowId);
      flow.updateNodeInternals([...nodeIds]);
    });
  });
}

function installHandleOrientationObserver() {
  let scheduled = false;
  const scheduleRefresh = () => {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(() => {
      scheduled = false;
      refreshHandleGeometry();
    });
  };

  const observer = new MutationObserver(scheduleRefresh);
  observer.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ['data-handlepos'],
  });
  scheduleRefresh();
}

export function installWorkflowArrowMarkers() {
  if (typeof document === 'undefined') return;
  installArrowDefinition();
  installHandleOrientationObserver();
}
