function numericOrder(field) {
  const value = Number(field?.sort_order);
  return Number.isFinite(value) ? value : Number.MAX_SAFE_INTEGER;
}

export function sortFieldsByOrder(fields) {
  return fields
    .map((field, index) => ({ field, index }))
    .sort((left, right) => {
      const orderDiff = numericOrder(left.field) - numericOrder(right.field);
      if (orderDiff) return orderDiff;
      if (left.field.kind !== right.field.kind) {
        return left.field.kind === "builtin" ? -1 : 1;
      }
      return left.index - right.index;
    })
    .map(({ field }) => field);
}

export function buildFieldOrderItems(fields) {
  return fields.map((field) =>
    field.kind === "builtin"
      ? { kind: "builtin", key: field.key }
      : { kind: "custom", id: Number(field.id) },
  );
}
