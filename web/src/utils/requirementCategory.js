export function buildCategoryOptions(items, uncategorizedLabel) {
  const result = [{ value: 0, label: uncategorizedLabel }];
  const children = new Map();
  for (const item of items || []) {
    const parentId = Number(item.parent_id) || 0;
    if (!children.has(parentId)) children.set(parentId, []);
    children.get(parentId).push(item);
  }
  const visited = new Set();
  const append = (parentId, depth) => {
    for (const item of children.get(parentId) || []) {
      const id = Number(item.id);
      if (!id || visited.has(id)) continue;
      visited.add(id);
      result.push({ value: id, label: `${"　".repeat(depth)}${item.name}` });
      append(id, depth + 1);
    }
  };
  append(0, 0);
  return result;
}
