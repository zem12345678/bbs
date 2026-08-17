import assert from "node:assert/strict";
import test from "node:test";
import {
  ROOT_FILE_FOLDER_ID,
  fileFolderMoveOptions,
  fileFolderOptionLabel,
  fileFolderParentPayload,
  focusedFileAfterDelete,
  focusedFileAfterUpdate,
  loadFileFolderTree,
  mergeKnownFileFolders,
  normalizeFileFolderId,
  withoutFocusedFileParam
} from "./fileFolders.js";

test("normalizes the root folder without losing precise folder ids", () => {
  assert.equal(normalizeFileFolderId(null), ROOT_FILE_FOLDER_ID);
  assert.equal(normalizeFileFolderId(0), ROOT_FILE_FOLDER_ID);
  assert.equal(normalizeFileFolderId("9223372036854775807"), "9223372036854775807");
  assert.equal(fileFolderParentPayload("0"), null);
  assert.equal(fileFolderParentPayload("9223372036854775807"), "9223372036854775807");
});

test("loads every paginated folder branch for move targets", async () => {
  const calls = [];
  const pages = new Map([
    ["0:0", { items: [{ id: "10", name: "设计" }], total: 2 }],
    ["0:1", { items: [{ id: "20", name: "归档" }], total: 2 }],
    ["10:0", { items: [{ id: "11", name: "封面" }], total: 1 }],
    ["20:0", { items: [], total: 0 }],
    ["11:0", { items: [], total: 0 }]
  ]);

  const folders = await loadFileFolderTree(async ({ parent_id: parentId, offset }) => {
    calls.push(`${parentId}:${offset}`);
    return pages.get(`${parentId}:${offset}`) || { items: [], total: 0 };
  }, { pageSize: 1 });

  assert.deepEqual(new Set(calls), new Set(["0:0", "0:1", "10:0", "20:0", "11:0"]));
  assert.deepEqual(folders.find((folder) => folder.id === "11").path, ["10", "11"]);
  assert.equal(fileFolderOptionLabel(folders.find((folder) => folder.id === "11"), folders), "设计 / 封面");
});

test("merges loaded folders with ancestry and excludes recursive move targets", () => {
  const rootChildren = mergeKnownFileFolders([], [
    { id: "10", name: "设计" },
    { id: "20", name: "归档" }
  ]);
  const designRoot = rootChildren.find((folder) => folder.id === "10");
  const allFolders = mergeKnownFileFolders(rootChildren, [{ id: "11", name: "封面" }], [designRoot]);
  const design = allFolders.find((folder) => folder.id === "10");
  const cover = allFolders.find((folder) => folder.id === "11");

  assert.deepEqual(cover.path, ["10", "11"]);
  assert.equal(fileFolderOptionLabel(cover, allFolders), "设计 / 封面");
  assert.deepEqual(
    fileFolderMoveOptions(allFolders, design.id).map((folder) => folder.id),
    ["20"]
  );
});

test("keeps the notification-focused file synchronized across edit, move, and delete", () => {
  const focused = {
    id: "9223372036854775807",
    original_name: "clips-old.json",
    folder_id: null,
    content_type: "application/json"
  };
  const updated = focusedFileAfterUpdate(focused, focused.id, {
    id: focused.id,
    original_name: "clips-new.json",
    folder_id: "9007199254740993"
  });

  assert.deepEqual(updated, {
    id: "9223372036854775807",
    original_name: "clips-new.json",
    folder_id: "9007199254740993",
    content_type: "application/json"
  });
  assert.equal(focusedFileAfterUpdate(focused, "123", { original_name: "unrelated.json" }), focused);
  assert.equal(focusedFileAfterDelete(updated, updated.id), null);
  assert.equal(focusedFileAfterDelete(updated, "123"), updated);
});

test("clears only the deleted focused file query parameter", () => {
  const searchParams = new URLSearchParams("file_id=9223372036854775807&source=notification");

  assert.equal(
    withoutFocusedFileParam(searchParams, "9223372036854775807").toString(),
    "source=notification"
  );
  assert.equal(
    withoutFocusedFileParam(searchParams, "123").toString(),
    "file_id=9223372036854775807&source=notification"
  );
  assert.equal(searchParams.toString(), "file_id=9223372036854775807&source=notification");
});
