// Stable color assignment for models

import type { ModelId } from './types';

const COLORS = ['#58a6ff', '#3fb950', '#d29922', '#f85149', '#bc8cff', '#79c0ff', '#56d364'];

const colorMap = new Map<ModelId, string>();

export function modelColor(model: ModelId, i?: number): string {
  if (!colorMap.has(model)) {
    const idx = i ?? colorMap.size;
    colorMap.set(model, COLORS[idx % COLORS.length]);
  }
  return colorMap.get(model)!;
}
