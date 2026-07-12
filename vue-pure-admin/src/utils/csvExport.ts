export type CsvColumn<T> = {
  header: string;
  value: (row: T) => unknown;
};

export function downloadCsv<T>(
  filename: string,
  columns: CsvColumn<T>[],
  rows: T[]
) {
  const csv = toCsv(columns, rows);
  const blob = new Blob(["\uFEFF", csv], {
    type: "text/csv;charset=utf-8"
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.style.display = "none";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function toCsv<T>(columns: CsvColumn<T>[], rows: T[]) {
  return [columns.map(column => escapeCsvValue(column.header)).join(",")]
    .concat(
      rows.map(row =>
        columns.map(column => escapeCsvValue(column.value(row))).join(",")
      )
    )
    .join("\r\n");
}

function escapeCsvValue(value: unknown) {
  const text = String(value ?? "");
  if (!/[",\r\n]/.test(text)) return text;
  return `"${text.replace(/"/g, '""')}"`;
}
