const paths = {
  folder: "M3 7V5a1 1 0 0 1 1-1h6l2 3h8a1 1 0 0 1 1 1v11H3Z",
  document: "M6 3h8l4 4v14H6ZM14 3v5h4M9 12h6M9 16h6",
  image: "M3 4h18v16H3ZM3 17l6-7 5 5 3-4 4 5M16 8h.01",
  archive: "M3 4h18v5H3ZM5 9v12h14V9M9 13h6",
  search: "M20 20l-5-5M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0Z",
  plus: "M12 5v14M5 12h14",
  upload: "M12 16V3m-5 5 5-5 5 5M4 15v6h16v-6",
  download: "M12 3v13m-5-5 5 5 5-5M4 17v4h16v-4",
  trash: "M4 6h16M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7M14 10v7",
  grid: "M3 3h7v7H3ZM14 3h7v7h-7ZM3 14h7v7H3ZM14 14h7v7h-7Z",
  list: "M8 5h13M8 12h13M8 19h13M3 5h.01M3 12h.01M3 19h.01",
  back: "M20 12H4m6-6-6 6 6 6",
  chevron: "m9 5 7 7-7 7",
  more: "M5 12h.01M12 12h.01M19 12h.01",
  clock: "M12 7v5l3 2M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z",
  close: "m6 6 12 12M6 18 18 6",
  move: "M3 7V4h7l3 3h8v14H3ZM7 14h10m-4-4 4 4-4 4",
  check: "m5 12 4 4L19 6",
  link: "m9 15 6-6M8 16l-1 1a4 4 0 0 1-6-6l4-4a4 4 0 0 1 6 0m2 1 1-1a4 4 0 0 1 6 6l-4 4a4 4 0 0 1-6 0",
  restore: "M3 10a9 9 0 1 1 1 8M3 3v7h7M12 7v5l4 2",
  info: "M12 11v6M12 7h.01M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0Z",
};
export default function ArchiveIcon({
  name = "document",
  size = 20,
  ...props
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.65"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    >
      <path d={paths[name] || paths.document} />
    </svg>
  );
}
