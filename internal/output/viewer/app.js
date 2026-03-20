const state = {
  manifest: window.CODEMAP_DATA ? window.CODEMAP_DATA.manifest : null,
  graphs: window.CODEMAP_DATA ? window.CODEMAP_DATA.graphs || {} : {},
  selectedProject: null,
  selectedNode: null,
  query: "",
  showChangedOnly: false,
  viewport: { scale: 1, offsetX: 0, offsetY: 0, isPanning: false },
  draggingNode: null,
  pointer: null,
  layoutByProject: {},
  globalEventsBound: false,
  focus: { target: null, start: 0, end: 0 },
};

const NODE_HEIGHT = 84;
const DRAG_THRESHOLD = 6;
const NODE_MIN_WIDTH = 140;
const NODE_MAX_WIDTH = 280;
const NODE_HORIZONTAL_PADDING = 28;

if (state.manifest && state.manifest.projects && state.manifest.projects[0]) {
  state.selectedProject = state.manifest.projects[0].id;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function getCurrentGraph() {
  if (!state.selectedProject) {
    return null;
  }
  return state.graphs[state.selectedProject] || null;
}

function ensureLayout(graph) {
  if (!graph) {
    return {};
  }
  const projectID = graph.project.id;
  const existing = state.layoutByProject[projectID] || {};
  const base = layoutNodes(graph.nodes.slice(0, 36));
  const merged = { ...base };
  Object.keys(existing).forEach((nodeID) => {
    merged[nodeID] = { ...merged[nodeID], ...existing[nodeID] };
  });
  state.layoutByProject[projectID] = merged;
  return state.layoutByProject[projectID];
}

function getFilteredGraph() {
  const graph = getCurrentGraph();
  if (!graph) {
    return null;
  }

  const normalizedQuery = state.query.trim().toLowerCase();
  const nodes = graph.nodes.filter((node) => {
    if (state.showChangedOnly && !node.changed) {
      return false;
    }
    if (!normalizedQuery) {
      return true;
    }
    return [node.label, node.path, node.language, node.type].filter(Boolean).some((value) =>
      String(value).toLowerCase().includes(normalizedQuery)
    );
  });

  const nodeSet = new Set(nodes.map((node) => node.id));
  const edges = graph.edges.filter((edge) => nodeSet.has(edge.source) && nodeSet.has(edge.target));
  return { project: graph.project, nodes, edges };
}

function getConnections(filteredGraph, node) {
  if (!filteredGraph || !node) {
    return { incoming: [], outgoing: [] };
  }
  const nodeById = Object.fromEntries(filteredGraph.nodes.map((item) => [item.id, item]));
  return {
    outgoing: filteredGraph.edges
      .filter((edge) => edge.source === node.id)
      .map((edge) => ({ edge, node: nodeById[edge.target] }))
      .filter((item) => item.node),
    incoming: filteredGraph.edges
      .filter((edge) => edge.target === node.id)
      .map((edge) => ({ edge, node: nodeById[edge.source] }))
      .filter((item) => item.node),
  };
}

function render() {
  const root = document.getElementById("root");
  if (!state.manifest) {
    root.innerHTML = `<main class="graph-panel panel empty-state">Viewer data was not embedded correctly. Re-run the analyzer to regenerate the artifact.</main>`;
    return;
  }

  const filteredGraph = getFilteredGraph();
  const layout = filteredGraph ? ensureLayout(filteredGraph) : {};
  const selectedNode = filteredGraph && state.selectedNode
    ? filteredGraph.nodes.find((node) => node.id === state.selectedNode) || null
    : null;
  const connections = getConnections(filteredGraph, selectedNode);

  root.innerHTML = `
    <div class="app-shell">
      ${renderSidebar()}
      ${renderGraphPanel(filteredGraph, layout)}
      ${renderDetailsPanel(selectedNode, connections)}
    </div>
  `;

  bindEvents(filteredGraph, layout);
  restoreFocus();
}

function renderSidebar() {
  const manifest = state.manifest;
  const projects = (manifest.projects || []).map((project) => `
    <button
      type="button"
      class="project-card${project.id === state.selectedProject ? " active" : ""}${project.changedNodes > 0 ? " changed" : ""}"
      data-project-id="${escapeHTML(project.id)}"
    >
      <strong>${escapeHTML(project.name)}</strong>
      <div class="meta">${escapeHTML(project.root)}</div>
      <div class="meta">${project.nodeCount} nodes • ${project.edgeCount} edges</div>
    </button>
  `).join("");

  return `
    <aside class="sidebar panel">
      <div class="hero">
        <div class="eyebrow">Codemap</div>
        <h1>Architecture graphs, split by project</h1>
        <p>Open a project, filter by path or node type, and inspect changed relationships directly from the PR artifact.</p>
      </div>
      <input class="search" id="search-input" value="${escapeHTML(state.query)}" placeholder="Search files, modules, classes" />
      <div class="filters">
        <label class="filter-chip">
          <input type="checkbox" id="changed-only-toggle" ${state.showChangedOnly ? "checked" : ""} />
          Changed nodes only
        </label>
      </div>
      <div class="summary">${manifest.projectCount} projects detected • ${manifest.graphCount} graphs generated</div>
      <div class="project-list">${projects}</div>
      <div class="legend">
        <div class="legend-item detail-block">
          <div class="detail-title">Node types</div>
          <div class="pill-row">
            ${["project", "directory", "file", "module", "symbol"].map((item) => `<span class="pill">${item}</span>`).join("")}
          </div>
        </div>
      </div>
    </aside>
  `;
}

function renderGraphPanel(graph, layout) {
  if (!graph) {
    return `<main class="graph-panel panel empty-state">No graph available for the selected filters.</main>`;
  }

  const canvasNodes = graph.nodes.slice(0, 36);
  const positions = layout;
  const nodeIndex = Object.fromEntries(canvasNodes.map((node) => [node.id, node]));
  const edges = graph.edges.filter((edge) => nodeIndex[edge.source] && nodeIndex[edge.target]).slice(0, 80);
  const changedCount = graph.nodes.filter((node) => node.changed).length;
  const world = getWorldBounds(canvasNodes, positions);
  const highlight = getHighlightState(canvasNodes, edges, state.selectedNode);

  const edgeLines = edges.map((edge) => {
    const source = positions[edge.source];
    const target = positions[edge.target];
    const classes = [
      "graph-edge",
      highlight.edgeIDs.has(edge.id) ? "is-highlighted" : "",
      highlight.hasSelection && !highlight.edgeIDs.has(edge.id) ? "is-dimmed" : "",
      edge.changed ? "changed" : "",
    ].filter(Boolean).join(" ");
    const stroke = edge.changed ? "rgba(194, 65, 12, 0.55)" : "rgba(15, 118, 110, 0.24)";
    const strokeWidth = highlight.edgeIDs.has(edge.id) ? 3.4 : (edge.changed ? 2.5 : 1.6);
    return `<line class="${classes}" x1="${source.x + source.width / 2}" y1="${source.y + NODE_HEIGHT / 2}" x2="${target.x + target.width / 2}" y2="${target.y + NODE_HEIGHT / 2}" stroke="${stroke}" stroke-width="${strokeWidth}" />`;
  }).join("");

  const nodeButtons = canvasNodes.map((node) => `
    <button
      type="button"
      class="node${node.id === state.selectedNode ? " selected" : ""}${highlight.neighborIDs.has(node.id) ? " neighbor" : ""}${highlight.hasSelection && !highlight.focusIDs.has(node.id) ? " is-dimmed" : ""}${node.changed ? " changed" : ""}${state.draggingNode === node.id ? " dragging" : ""}"
      style="left:${positions[node.id].x}px; top:${positions[node.id].y}px; width:${positions[node.id].width}px;"
      data-node-id="${escapeHTML(node.id)}"
    >
      <span class="type-pill">${escapeHTML(node.type)}</span>
      <div class="node-label">${escapeHTML(node.label)}</div>
    </button>
  `).join("");

  return `
    <main class="graph-panel panel">
      <div class="graph-header">
        <div>
          <div class="eyebrow">Project map</div>
          <h2>${escapeHTML(graph.project.name)}</h2>
          <div class="meta">${escapeHTML(graph.project.root)}</div>
        </div>
        <div class="stats">
          ${renderStatCard("Nodes", graph.nodes.length)}
          ${renderStatCard("Edges", graph.edges.length)}
          ${renderStatCard("Changed", changedCount)}
        </div>
      </div>
      <div class="graph-stage">
        <div class="graph-toolbar meta">Scroll to zoom, drag the background to pan, and drag nodes to rearrange the map.</div>
        <div class="canvas${state.viewport.isPanning ? " is-panning" : ""}" id="graph-canvas">
          <div class="viewport" id="graph-viewport" style="width:${world.width}px; height:${world.height}px; transform: translate(${state.viewport.offsetX}px, ${state.viewport.offsetY}px) scale(${state.viewport.scale});">
            <svg class="edge-layer" width="${world.width}" height="${world.height}" viewBox="0 0 ${world.width} ${world.height}" preserveAspectRatio="none">${edgeLines}</svg>
            <div class="node-layer" style="width:${world.width}px; height:${world.height}px;">${nodeButtons}</div>
          </div>
        </div>
      </div>
    </main>
  `;
}

function renderDetailsPanel(node, connections) {
  return `
    <aside class="details panel">
      <div class="hero">
        <div class="eyebrow">Inspector</div>
        <h1>${escapeHTML(node ? node.label : "Select a node")}</h1>
        <p>${escapeHTML(node ? "Inspect connected nodes and metadata here." : "Use the graph canvas or project search to inspect files, modules, and symbols.")}</p>
      </div>
      ${node ? `
        <div class="detail-block">
          <div class="detail-title">Node metadata</div>
          <div class="pill-row">
            ${[node.type, node.language || "unknown", node.changed ? "changed" : "unchanged"].map((item) => `<span class="pill">${escapeHTML(item)}</span>`).join("")}
          </div>
          <div class="meta" style="margin-top:10px;">${escapeHTML(node.path || "No path metadata available.")}</div>
        </div>
      ` : ""}
      ${renderConnectionList("Outgoing edges", connections.outgoing)}
      ${renderConnectionList("Incoming edges", connections.incoming)}
    </aside>
  `;
}

function renderConnectionList(title, items) {
  if (!items.length) {
    return `
      <div class="detail-block">
        <div class="detail-title">${escapeHTML(title)}</div>
        <div class="meta">No connections in the current filtered view.</div>
      </div>
    `;
  }

  return `
    <div class="detail-block">
      <div class="detail-title">${escapeHTML(title)}</div>
      <div class="edge-list">
        ${items.slice(0, 10).map(({ edge, node }) => `
          <div class="list-item${edge.changed ? " changed" : ""}">
            <strong>${escapeHTML(node.label)}</strong>
            <div class="meta">${escapeHTML(`${edge.type} • ${node.path || node.type}`)}</div>
          </div>
        `).join("")}
      </div>
    </div>
  `;
}

function renderStatCard(label, value) {
  return `<div class="stat-card"><strong>${value}</strong>${escapeHTML(label)}</div>`;
}

function bindEvents(filteredGraph, layout) {
  const searchInput = document.getElementById("search-input");
  if (searchInput) {
    searchInput.addEventListener("input", (event) => {
      state.focus = {
        target: "search-input",
        start: event.target.selectionStart || 0,
        end: event.target.selectionEnd || 0,
      };
      state.query = event.target.value;
      state.selectedNode = null;
      render();
    });
  }

  const toggle = document.getElementById("changed-only-toggle");
  if (toggle) {
    toggle.addEventListener("change", (event) => {
      state.showChangedOnly = event.target.checked;
      state.selectedNode = null;
      render();
    });
  }

  document.querySelectorAll("[data-project-id]").forEach((button) => {
    button.addEventListener("click", () => {
      state.selectedProject = button.getAttribute("data-project-id");
      state.selectedNode = null;
      render();
    });
  });

  document.querySelectorAll("[data-node-id]").forEach((button) => {
    button.addEventListener("pointerdown", (event) => {
      event.stopPropagation();
      const nodeID = button.getAttribute("data-node-id");
      state.pointer = {
        kind: "node",
        id: nodeID,
        startX: event.clientX,
        startY: event.clientY,
        originX: layout[nodeID].x,
        originY: layout[nodeID].y,
        moved: false,
      };
      button.setPointerCapture(event.pointerId);
    });
  });

  const canvas = document.getElementById("graph-canvas");
  if (canvas) {
    canvas.addEventListener("wheel", (event) => {
      event.preventDefault();
      const rect = canvas.getBoundingClientRect();
      const pointerX = event.clientX - rect.left;
      const pointerY = event.clientY - rect.top;
      const worldX = (pointerX - state.viewport.offsetX) / state.viewport.scale;
      const worldY = (pointerY - state.viewport.offsetY) / state.viewport.scale;
      const delta = event.deltaY < 0 ? 0.1 : -0.1;
      const nextScale = clamp(state.viewport.scale + delta, 0.45, 2.2);
      state.viewport.offsetX = pointerX - worldX * nextScale;
      state.viewport.offsetY = pointerY - worldY * nextScale;
      state.viewport.scale = nextScale;
      render();
    }, { passive: false });

    canvas.addEventListener("pointerdown", (event) => {
      if (event.target.closest("[data-node-id]")) {
        return;
      }
      state.viewport.isPanning = true;
      state.pointer = {
        kind: "pan",
        startX: event.clientX,
        startY: event.clientY,
        originX: state.viewport.offsetX,
        originY: state.viewport.offsetY,
        moved: false,
      };
      canvas.setPointerCapture(event.pointerId);
      render();
    });
  }

  bindGlobalEventsOnce();

  if (filteredGraph && state.selectedNode && !filteredGraph.nodes.find((node) => node.id === state.selectedNode)) {
    state.selectedNode = null;
    render();
  }
}

function handlePointerMove(event) {
  if (!state.pointer) {
    return;
  }

  if (state.pointer.kind === "pan") {
    const deltaX = event.clientX - state.pointer.startX;
    const deltaY = event.clientY - state.pointer.startY;
    const movedEnough = Math.abs(deltaX) > DRAG_THRESHOLD || Math.abs(deltaY) > DRAG_THRESHOLD;
    if (!state.pointer.moved && !movedEnough) {
      return;
    }
    state.pointer.moved = true;
    state.viewport.offsetX = state.pointer.originX + deltaX;
    state.viewport.offsetY = state.pointer.originY + deltaY;
    render();
    return;
  }

  if (state.pointer.kind === "node") {
    const graph = getFilteredGraph();
    if (!graph) {
      return;
    }
    const layout = ensureLayout(graph);
    const canvas = document.getElementById("graph-canvas");
    const rect = canvas ? canvas.getBoundingClientRect() : { width: 960, height: 560 };
    const deltaX = event.clientX - state.pointer.startX;
    const deltaY = event.clientY - state.pointer.startY;
    const movedEnough = Math.abs(deltaX) > DRAG_THRESHOLD || Math.abs(deltaY) > DRAG_THRESHOLD;
    if (!state.pointer.moved && !movedEnough) {
      return;
    }
    state.pointer.moved = true;
    state.draggingNode = state.pointer.id;
    const nextX = state.pointer.originX + deltaX / state.viewport.scale;
    const nextY = state.pointer.originY + deltaY / state.viewport.scale;
    const visibleMinX = (-state.viewport.offsetX) / state.viewport.scale;
    const visibleMinY = (-state.viewport.offsetY) / state.viewport.scale;
    const nodeWidth = layout[state.pointer.id].width || NODE_MIN_WIDTH;
    const visibleMaxX = (rect.width - state.viewport.offsetX) / state.viewport.scale - nodeWidth;
    const visibleMaxY = (rect.height - state.viewport.offsetY) / state.viewport.scale - NODE_HEIGHT;
    layout[state.pointer.id] = {
      x: clamp(nextX, visibleMinX + 8, Math.max(visibleMinX + 8, visibleMaxX - 8)),
      y: clamp(nextY, visibleMinY + 8, Math.max(visibleMinY + 8, visibleMaxY - 8)),
    };
    render();
  }
}

function handlePointerUp() {
  if (!state.pointer && !state.viewport.isPanning && !state.draggingNode) {
    return;
  }
  if (state.pointer && state.pointer.kind === "node" && !state.pointer.moved) {
    const nextNode = state.pointer.id;
    state.selectedNode = state.selectedNode === nextNode ? null : nextNode;
  }
  if (state.pointer && state.pointer.kind === "pan" && !state.pointer.moved) {
    state.selectedNode = null;
  }
  state.pointer = null;
  state.viewport.isPanning = false;
  state.draggingNode = null;
  render();
}

function bindGlobalEventsOnce() {
  if (state.globalEventsBound) {
    return;
  }
  document.addEventListener("pointermove", handlePointerMove);
  document.addEventListener("pointerup", handlePointerUp);
  document.addEventListener("pointercancel", handlePointerUp);
  state.globalEventsBound = true;
}

function restoreFocus() {
  if (!state.focus.target) {
    return;
  }
  const element = document.getElementById(state.focus.target);
  if (!element) {
    return;
  }
  element.focus();
  if (typeof element.setSelectionRange === "function") {
    element.setSelectionRange(state.focus.start, state.focus.end);
  }
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function layoutNodes(nodes) {
  const columns = 4;
  const cardWidth = 210;
  const cardHeight = 110;
  const positions = {};
  nodes.forEach((node, index) => {
    const column = index % columns;
    const row = Math.floor(index / columns);
    const width = getNodeWidth(node);
    positions[node.id] = {
      x: 36 + column * cardWidth + (row % 2 === 0 ? 0 : 36),
      y: 36 + row * cardHeight,
      width,
    };
  });
  return positions;
}

function getWorldBounds(nodes, positions) {
  let maxX = 960;
  let maxY = 560;
  nodes.forEach((node) => {
    const position = positions[node.id];
    if (!position) {
      return;
    }
    maxX = Math.max(maxX, position.x + position.width + 40);
    maxY = Math.max(maxY, position.y + NODE_HEIGHT + 40);
  });
  return { width: maxX, height: maxY };
}

function getNodeWidth(node) {
  const textWidth = String(node.label || "").length * 8.5 + NODE_HORIZONTAL_PADDING;
  return clamp(Math.round(textWidth), NODE_MIN_WIDTH, NODE_MAX_WIDTH);
}

function getHighlightState(nodes, edges, selectedNodeID) {
  const state = {
    hasSelection: false,
    focusIDs: new Set(),
    neighborIDs: new Set(),
    edgeIDs: new Set(),
  };

  if (!selectedNodeID) {
    return state;
  }

  const nodeSet = new Set(nodes.map((node) => node.id));
  if (!nodeSet.has(selectedNodeID)) {
    return state;
  }

  state.hasSelection = true;
  state.focusIDs.add(selectedNodeID);

  edges.forEach((edge) => {
    if (edge.source === selectedNodeID || edge.target === selectedNodeID) {
      state.edgeIDs.add(edge.id);
      state.focusIDs.add(edge.source);
      state.focusIDs.add(edge.target);
      if (edge.source !== selectedNodeID) {
        state.neighborIDs.add(edge.source);
      }
      if (edge.target !== selectedNodeID) {
        state.neighborIDs.add(edge.target);
      }
    }
  });

  return state;
}

render();
