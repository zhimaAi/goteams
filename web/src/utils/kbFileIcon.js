import {
  FileExcelFilled,
  FileImageFilled,
  FileMarkdownFilled,
  FilePdfFilled,
  FilePptFilled,
  FileTextFilled,
  FileUnknownFilled,
  FileWordFilled,
  FileZipFilled,
  FolderFilled,
  Html5Filled,
  VideoCameraFilled,
} from '@ant-design/icons-vue'

// File extension → file type icon groups (order matters, first match wins).
//
// Only the file types present in the Figma design (GoTeams 2676:11316 tree area)
// get a dedicated glyph. Extensions NOT covered by the design (e.g. audio, JSON,
// YAML, code files …) intentionally fall through to the "unknown file" glyph
// (FileUnknownFilled), matching the design's last tree row "未知文件.???".
const KB_DOC_ICON_GROUPS = [
  { key: 'word', exts: ['doc', 'docx'], icon: FileWordFilled },
  { key: 'excel', exts: ['xls', 'xlsx'], icon: FileExcelFilled },
  { key: 'csv', exts: ['csv'], icon: FileExcelFilled },
  { key: 'ppt', exts: ['ppt', 'pptx'], icon: FilePptFilled },
  { key: 'pdf', exts: ['pdf'], icon: FilePdfFilled },
  { key: 'ofd', exts: ['ofd'], icon: FilePdfFilled },
  {
    key: 'image',
    exts: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'avif'],
    icon: FileImageFilled,
  },
  { key: 'markdown', exts: ['md', 'markdown'], icon: FileMarkdownFilled },
  {
    key: 'zip',
    exts: ['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz'],
    icon: FileZipFilled,
  },
  {
    key: 'video',
    exts: ['mp4', 'mov', 'avi', 'mkv', 'webm', 'flv', 'wmv', 'm4v'],
    icon: VideoCameraFilled,
  },
  {
    key: 'txt',
    exts: ['txt', 'log', 'text'],
    icon: FileTextFilled,
  },
  {
    key: 'html',
    exts: ['html', 'htm'],
    icon: Html5Filled,
  },
]

// Folder icon is exported separately because folders don't go through the
// extension lookup.
export const KB_FOLDER_ICON = FolderFilled

// kbDocFileIcon returns the icon component and a modifier class for a knowledge
// item's document. Anything outside KB_DOC_ICON_GROUPS falls back to the unknown
// file glyph (grey FileUnknownFilled), matching the design's "未知文件" row.
// Newly created structured documents carry no source extension — they default
// to the docx glyph (blue Word file) instead of the unknown placeholder.
export function kbDocFileIcon(item) {
  const ext = String(item?.source_extension || '').toLowerCase()
  const group = KB_DOC_ICON_GROUPS.find((g) => g.exts.includes(ext))
  if (group) {
    return { icon: group.icon, modifier: `kb-doc-icon--${group.key}` }
  }
  if (item?.item_type === 'document' && !ext) {
    return { icon: FileWordFilled, modifier: 'kb-doc-icon--word' }
  }
  return { icon: FileUnknownFilled, modifier: 'kb-doc-icon--unknown' }
}