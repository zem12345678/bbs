import { toId } from "./formatters.js";
import { listItems, listTotal } from "./apiShapes.js";

export const ROOT_FILE_FOLDER_ID = "0";

export function normalizeFileFolderId(value) {
  const id = toId(value);
  return id && id !== ROOT_FILE_FOLDER_ID ? id : ROOT_FILE_FOLDER_ID;
}

export function fileFolderParentPayload(value) {
  const id = normalizeFileFolderId(value);
  return id === ROOT_FILE_FOLDER_ID ? null : id;
}

export function focusedFileAfterUpdate(current, fileId, updated) {
  const targetId = toId(fileId);
  if (!targetId || toId(current?.id) !== targetId || !updated) return current;
  return { ...current, ...updated, id: toId(updated?.id) || targetId };
}

export function focusedFileAfterDelete(current, fileId) {
  return toId(current?.id) === toId(fileId) ? null : current;
}

export function withoutFocusedFileParam(searchParams, fileId) {
  const next = new URLSearchParams(searchParams);
  if (toId(next.get("file_id")) === toId(fileId)) next.delete("file_id");
  return next;
}

export function mergeKnownFileFolders(current = [], folders = [], ancestors = []) {
  const byId = new Map(current.map((folder) => [normalizeFileFolderId(folder?.id), folder]));
  const ancestorIds = ancestors
    .map((folder) => normalizeFileFolderId(folder?.id))
    .filter((id) => id !== ROOT_FILE_FOLDER_ID);

  folders.forEach((folder) => {
    const id = normalizeFileFolderId(folder?.id);
    if (id === ROOT_FILE_FOLDER_ID) return;
    byId.set(id, {
      ...byId.get(id),
      ...folder,
      id,
      path: [...ancestorIds, id]
    });
  });

  return Array.from(byId.values()).sort((left, right) =>
    String(left?.name || "").localeCompare(String(right?.name || ""), "zh-CN")
  );
}

export function fileFolderMoveOptions(knownFolders = [], movingFolderId = "") {
  const movingId = normalizeFileFolderId(movingFolderId);
  return knownFolders.filter((folder) => {
    const id = normalizeFileFolderId(folder?.id);
    if (id === ROOT_FILE_FOLDER_ID || movingId === ROOT_FILE_FOLDER_ID) return id !== ROOT_FILE_FOLDER_ID;
    return id !== movingId && !(folder?.path || []).map(normalizeFileFolderId).includes(movingId);
  });
}

export function fileFolderOptionLabel(folder, knownFolders = [], folderById) {
  const lookup = folderById || new Map(knownFolders.map((item) => [normalizeFileFolderId(item?.id), item]));
  const path = (folder?.path || [folder?.id])
    .map((id) => lookup.get(normalizeFileFolderId(id))?.name)
    .filter(Boolean);
  return path.join(" / ") || String(folder?.name || "未命名文件夹");
}

export async function loadFileFolderTree(loadPage, { pageSize = 100, maxFolders = 5000 } = {}) {
  const queue = [{ id: ROOT_FILE_FOLDER_ID, path: [] }];
  const folders = [];
  const seen = new Set();

  while (queue.length > 0) {
    const parent = queue.shift();
    let offset = 0;
    while (true) {
      const data = await loadPage({ parent_id: parent.id, limit: pageSize, offset });
      const pageItems = listItems(data);
      for (const item of pageItems) {
        const id = normalizeFileFolderId(item?.id);
        if (id === ROOT_FILE_FOLDER_ID || seen.has(id)) continue;
        seen.add(id);
        const folder = { ...item, id, path: [...parent.path, id] };
        folders.push(folder);
        queue.push({ id, path: folder.path });
        if (folders.length > maxFolders) {
          throw new Error("文件夹数量超过可管理上限");
        }
      }
      offset += pageItems.length;
      if (pageItems.length === 0 || offset >= listTotal(data, pageItems)) break;
    }
  }

  const folderById = new Map(folders.map((folder) => [folder.id, folder]));
  return folders.sort((left, right) =>
    fileFolderOptionLabel(left, folders, folderById).localeCompare(fileFolderOptionLabel(right, folders, folderById), "zh-CN")
  );
}
