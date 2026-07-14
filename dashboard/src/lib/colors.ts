// Evenly spaced HSL colors scale to any number of models without recycling a
// small fixed palette. The caller supplies position and total after sorting.

import type { ModelId } from './types';

export function modelColor(_model: ModelId, index = 0, total = 1): string {
  const count = Math.max(1, total);
  const hue = Math.round((index / count) * 360);
  return `hsl(${hue} 68% 62%)`;
}
