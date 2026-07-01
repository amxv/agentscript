export const siteConfig = {
  name: "agentscript",
  strapline: "Readable agent transcripts in your terminal",
  description:
    "Documentation for agentscript, a Go CLI for opening, searching, filtering, and slicing Claude Code and Codex JSONL transcripts.",
  repoUrl: "https://github.com/amxv/agentscript",
  footerSections: [
    {
      title: "agentscript",
      text:
        "A terminal-first transcript reader for Claude Code and Codex sessions with stable block indexes and scriptable filters."
    },
    {
      title: "What this site covers",
      text:
        "Opening transcripts, searching sessions, hiding noisy blocks, slicing by stable indexes, and exporting readable context."
    },
    {
      title: "Repository",
      linkPrefix: "Source: ",
      linkHref: "https://github.com/amxv/agentscript",
      linkLabel: "github.com/amxv/agentscript"
    }
  ]
} as const;

export const docCategories = [
  "Start",
  "Workflows",
  "Reference"
] as const;

export const primaryNav = [
  { href: "/", label: "Overview" },
  { href: "/docs", label: "Docs" },
  { href: siteConfig.repoUrl, label: "GitHub", external: true }
];
