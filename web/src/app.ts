declare const React: any;
declare const ReactDOM: any;

type ViewerManifest = {
  repoRoot: string;
  generatedAt: string;
  projectCount: number;
  graphCount: number;
  changedFiles?: string[];
  projects: Array<{
    id: string;
    name: string;
    root: string;
    graphPath: string;
    nodeCount: number;
    edgeCount: number;
    changedNodes: number;
  }>;
};

type Node = {
  id: string;
  label: string;
  type: string;
  language?: string;
  path?: string;
  changed?: boolean;
};

type Edge = {
  id: string;
  source: string;
  target: string;
  type: string;
  changed?: boolean;
};

type Graph = {
  project: { id: string; name: string; root: string };
  nodes: Node[];
  edges: Edge[];
};

const { createElement: h } = React;

async function loadJSON<T>(path: string): Promise<T> {
  const response = await fetch(path);
  if (!response.ok) {
    throw new Error(`Failed to load ${path}`);
  }
  return response.json();
}

function bootstrap(): void {
  ReactDOM.createRoot(document.getElementById("root")).render(
    h("div", null, "Source mirror for the prebuilt viewer bundle in internal/output/viewer.")
  );
}

bootstrap();
