// @ts-check

const sidebars = {
  operatorSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Operate',
      collapsed: false,
      items: [
        'quick-start',
        'configuration',
        'manual-reviews',
        'webhook-policy',
        'docker',
        'operator-cli',
      ],
    },
    {
      type: 'category',
      label: 'Understand',
      collapsed: false,
      items: ['architecture', 'integrations'],
    },
    {
      type: 'category',
      label: 'Recover',
      collapsed: false,
      items: ['troubleshooting'],
    },
  ],
};

module.exports = sidebars;
