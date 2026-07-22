export const siteConfig = {
  name: "agentscript",
  strapline: "Readable agent transcripts in your terminal",
  description:
    "Documentation for agentscript, a Go CLI for opening, searching, filtering, slicing, splitting, exporting, and auditing Claude Code and Codex JSONL transcripts.",
  repoUrl: "https://github.com/amxv/agentscript",
  accentColor: "#6d28d9",
  accentColorDark: "#c4b5fd",
  footerSections: [
    {
      title: "agentscript",
      text:
        "A terminal-first transcript reader for Claude Code and Codex sessions with stable block indexes, turn slices, profiles, folding, and activity views."
    },
    {
      title: "What this site covers",
      text:
        "Opening transcripts, searching sessions, hiding noisy blocks, slicing by stable indexes or turns, extracting files/commands/activity, and exporting readable context."
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
  { href: "/docs", label: "Docs" },
  { href: siteConfig.repoUrl, label: "GitHub", external: true }
];
