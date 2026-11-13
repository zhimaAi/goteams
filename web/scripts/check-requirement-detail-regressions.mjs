import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const source = await readFile(
  new URL(
    "../src/components/common/RequirementDetailDrawer.vue",
    import.meta.url,
  ),
  "utf8",
);

// Property editors unmount as soon as editingProperty is cleared. Ending the
// edit on the input blur races with the option change event and drops the
// selection, so every select must end editing when its dropdown closes.
const selects = source.match(/<a-select\b[\s\S]*?<\/a-select>/g) || [];
assert.ok(selects.length >= 6, "property select editors must exist");
for (const select of selects) {
  assert.doesNotMatch(
    select,
    /@blur=/,
    "a select must not unmount on blur before its change event is handled",
  );
  assert.match(
    select,
    /@dropdownVisibleChange="\s*\(open\) => !open && \(editingProperty = ''\)\s*"/,
    "closing a select dropdown must end property editing",
  );
}

// Reloading the server detail must never clobber an unsaved editor draft.
// The guard lives inside loadCurrentDetail so every call site is covered.
assert.match(
  source,
  /async function loadCurrentDetail\(\) \{[\s\S]*?if \(!editingTitle\.value && !descriptionEditing\.value\) \{/,
  "detail reload must preserve an active editor draft",
);
assert.doesNotMatch(
  source,
  /preserveEditorDraft/,
  "draft protection must not depend on call sites passing a flag",
);

// Version rows arrive from the API as strings while the update endpoint takes
// a numeric id; sending the raw string is rejected with 400.
assert.match(
  source,
  /versions\.value = \(configs\?\.versions \|\| \[\]\)\.map\(\(item\) => \(\{[\s\S]*?id: Number\(item\.id\),/,
  "version options must carry numeric ids",
);

console.log("requirement detail regressions: ok");
