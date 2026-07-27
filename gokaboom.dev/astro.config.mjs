import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'
import starlightBlog from 'starlight-blog'

export default defineConfig({
  site: 'https://gokaboom.dev',
  integrations: [
    starlight({
      title: 'KaBOOM!',
      description: 'Build web apps faster with interactive design, coding, and debugging.',
      logo: {
        src: './src/assets/logo.svg',
        alt: 'KaBOOM!'
      },
      favicon: '/images/logo.png',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP'
        }
      ],
      customCss: ['./src/styles/custom.css'],
      components: {
        Head: './src/components/Head.astro',
        Header: './src/components/Header.astro',
        Footer: './src/components/Footer.astro',
        ThemeProvider: './src/components/ThemeProvider.astro',
        ThemeSelect: './src/components/ThemeSelect.astro'
      },
      plugins: [
        starlightBlog({
          title: 'Blog',
          prefix: 'blog',
          postsPerPage: 50,
          authors: {
            brenn: {
              name: 'Brenn Hill'
            },
            brennhill: {
              name: 'Brenn Hill'
            }
          }
        })
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Downloads', slug: 'downloads' },
            { label: 'Quick Start', slug: 'getting-started' },
            { label: 'Features', slug: 'features' },
            { label: 'Discover Workflows', slug: 'discover-workflows' },
            { label: 'Articles', slug: 'articles' },
            { label: 'Blog', link: '/blog/' }
          ]
        },
        {
          label: 'MCP Integration',
          items: [
            { label: 'Overview', slug: 'mcp-integration' },
            { label: 'Claude Code', slug: 'mcp-integration/claude-code' },
            { label: 'OpenAI Codex', slug: 'mcp-integration/codex' },
            { label: 'Cursor', slug: 'mcp-integration/cursor' },
            { label: 'Windsurf', slug: 'mcp-integration/windsurf' },
            { label: 'Claude Desktop', slug: 'mcp-integration/claude-desktop' },
            { label: 'Zed', slug: 'mcp-integration/zed' },
            { label: 'Gemini CLI', slug: 'mcp-integration/gemini' },
            { label: 'OpenCode', slug: 'mcp-integration/opencode' },
            { label: 'Antigravity', slug: 'mcp-integration/antigravity' }
          ]
        },
        {
          label: 'Tool Reference',
          items: [
            { label: 'Reference Overview', slug: 'reference' },
            { label: 'observe()', slug: 'reference/observe' },
            { label: 'analyze()', slug: 'reference/analyze' },
            { label: 'generate()', slug: 'reference/generate' },
            { label: 'configure()', slug: 'reference/configure' },
            { label: 'interact()', slug: 'reference/interact' },
            { label: 'Observe Examples', slug: 'reference/examples/observe-examples' },
            { label: 'Analyze Examples', slug: 'reference/examples/analyze-examples' },
            { label: 'Generate Examples', slug: 'reference/examples/generate-examples' },
            { label: 'Configure Examples', slug: 'reference/examples/configure-examples' },
            { label: 'Interact Examples', slug: 'reference/examples/interact-examples' },
            { label: 'Script Execution', slug: 'execute-scripts' }
          ]
        },
        {
          label: 'Guides',
          items: [
            { label: 'Start Here by Role', slug: 'guides/start-here-by-role' },
            { label: 'Engineering Track', slug: 'guides/tracks/engineering' },
            { label: 'QA Track', slug: 'guides/tracks/qa' },
            { label: 'Product Track', slug: 'guides/tracks/product' },
            { label: 'Support Track', slug: 'guides/tracks/support' },
            { label: 'Security Track', slug: 'guides/tracks/security' },
            { label: 'SEO Analysis', slug: 'guides/seo-analysis' },
            { label: 'Annotation + Skills + Terminal', slug: 'guides/annotation-skill-terminal-workflow' },
            { label: 'Product Demos', slug: 'guides/product-demos' },
            { label: 'Demo Scripts', slug: 'guides/demo-scripts' },
            { label: 'Debug Web Apps', slug: 'guides/debug-webapps' },
            { label: 'Security Auditing', slug: 'guides/security-auditing' },
            { label: 'Performance', slug: 'guides/performance' },
            { label: 'Accessibility', slug: 'guides/accessibility' },
            { label: 'WebSocket Debugging', slug: 'guides/websocket-debugging' },
            { label: 'API Validation', slug: 'guides/api-validation' },
            { label: 'Noise Filtering', slug: 'guides/noise-filtering' },
            { label: 'Resilient UAT Scripts', slug: 'guides/resilient-uat' },
            { label: 'Replace Selenium', slug: 'guides/replace-selenium' },
            { label: 'Automate & Notify', slug: 'guides/automate-and-notify' },
            { label: 'Visual Evidence Standards', slug: 'guides/visual-evidence-standards' }
          ]
        },
        {
          label: 'More',
          items: [
            { label: 'Architecture', slug: 'architecture' },
            { label: 'Security', slug: 'security' },
            { label: 'Troubleshooting', slug: 'troubleshooting' },
            { label: 'Agent Install Guide', slug: 'agent-install-guide' },
            { label: 'Alternatives', slug: 'alternatives' }
          ]
        }
      ]
    })
  ]
})
