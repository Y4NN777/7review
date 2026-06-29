// @ts-check

const config = {
  title: '7review',
  tagline: 'Operator documentation for controlled pull request and merge request reviews.',
  url: 'https://y4nn777.github.io',
  baseUrl: '/7review/',
  organizationName: 'Y4NN777',
  projectName: '7review',
  trailingSlash: false,
  onBrokenLinks: 'throw',
  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'fr'],
    localeConfigs: {
      en: {label: 'English'},
      fr: {label: 'Francais'},
    },
  },
  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },
  themes: ['@docusaurus/theme-mermaid'],
  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.js',
          routeBasePath: 'docs',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      },
    ],
  ],
  themeConfig: {
    image: 'img/7review-console-card.svg',
    navbar: {
      title: '7review',
      items: [
        {type: 'docSidebar', sidebarId: 'operatorSidebar', position: 'left', label: 'Docs'},
        {to: '/docs/manual-reviews', label: 'Manual Reviews', position: 'left'},
        {to: '/docs/webhook-policy', label: 'Webhook Policy', position: 'left'},
        {type: 'localeDropdown', position: 'right'},
        {href: 'https://github.com/Y4NN777/7review', label: 'GitHub', position: 'right'},
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          title: 'Operate',
          items: [
            {label: 'Quick Start', to: '/docs/quick-start'},
            {label: 'Manual Reviews', to: '/docs/manual-reviews'},
            {label: 'Troubleshooting', to: '/docs/troubleshooting'},
          ],
        },
        {
          title: 'Understand',
          items: [
            {label: 'Architecture', to: '/docs/architecture'},
            {label: 'Integrations', to: '/docs/integrations'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: 'https://github.com/Y4NN777/7review'},
          ],
        },
      ],
      copyright: `Copyright (c) ${new Date().getFullYear()} 7review contributors.`,
    },
    prism: {
      theme: require('prism-react-renderer').themes.github,
      darkTheme: require('prism-react-renderer').themes.nightOwl,
      additionalLanguages: ['bash', 'json', 'yaml', 'go'],
    },
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: false,
    },
  },
};

module.exports = config;
