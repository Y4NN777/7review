import React from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './index.module.css';

const copy = {
  en: {
    title: '7review operator documentation',
    description: '7review operator documentation for GitHub pull request and GitLab merge request reviews',
    eyebrow: 'Agentic review pipeline for GitHub PRs and GitLab MRs',
    heading: 'A code review agent that works from real change context.',
    lede: '7review takes an exact PR or MR, enriches it with SCM metadata and diffs, selects repository knowledge, skills, and memory, routes model review through configured providers, validates findings, publishes review output, and waits for human approval before final publication.',
    start: 'Start with quick setup',
    architecture: 'View architecture',
    loopTitle: 'Agentic review loop',
    loopIntro: 'The operator chooses the PR or MR. The agent handles enrichment, context assembly, model review, validation, publishing state, memory, and final human approval gates.',
    console: 'Operator commands',
    footer: 'Final publishing requires human approval',
    checks: ['SCM enrichment', 'Context and memory recall', 'Model review with tools', 'Human approval gate'],
    loop: [
      ['01', 'Ingest', 'A manual request or policy-gated webhook enters the bounded worker queue.'],
      ['02', 'Understand', 'The agent loads SCM metadata, diffs, repository knowledge, skills, and approved memory.'],
      ['03', 'Review', 'Configured model providers review the assembled context with governed read-only tool access.'],
      ['04', 'Approve', 'Validators classify findings, publish review output, and wait for human approval before final publication.'],
    ],
    routes: [
      ['Manual Reviews', '/docs/manual-reviews', 'Request one GitHub PR or GitLab MR through the same bounded queue used by webhooks.'],
      ['Webhook Policy', '/docs/webhook-policy', 'Control automatic review with mode, label, project, repository, and branch rules.'],
      ['Docker Runtime', '/docs/docker', 'Run the agent with Headroom and MemPalace on the private Compose network.'],
      ['Troubleshooting', '/docs/troubleshooting', 'Diagnose queue pressure, token failures, sidecar readiness, and publish state.'],
    ],
  },
  fr: {
    title: 'Documentation opérateur',
    description: 'Documentation opérateur 7review pour les pull requests GitHub et merge requests GitLab',
    eyebrow: 'Pipeline de review agentique pour PR GitHub et MR GitLab',
    heading: 'Un agent de review qui travaille depuis le vrai contexte du changement.',
    lede: '7review prend une PR ou MR précise, l’enrichit avec les métadonnées SCM et les diffs, sélectionne la connaissance du dépôt, les skills et la mémoire, route la review vers les modèles configurés, valide les findings, publie la review et attend l’approbation humaine avant la publication finale.',
    start: 'Commencer le setup',
    architecture: 'Voir architecture',
    loopTitle: 'Boucle de review agentique',
    loopIntro: 'L’opérateur choisit la PR ou la MR. L’agent gère l’enrichissement, l’assemblage du contexte, la review modèle, la validation, l’état de publication, la mémoire et les points de contrôle d’approbation finale.',
    console: 'Commandes opérateur',
    footer: 'La publication finale exige une approbation humaine',
    checks: ['Enrichissement SCM', 'Contexte et mémoire', 'Review modèle avec outils', 'Approbation humaine'],
    loop: [
      ['01', 'Ingest', 'Une demande manuelle ou un webhook filtré par politique entre dans la file de travail bornée.'],
      ['02', 'Understand', 'L’agent charge les métadonnées SCM, les diffs, la connaissance du dépôt, les skills et la mémoire approuvée.'],
      ['03', 'Review', 'Les modèles configurés analysent le contexte assemblé avec des outils read-only gouvernés.'],
      ['04', 'Approve', 'Les validateurs classent les findings, publient la review et attendent l’approbation humaine avant le final.'],
    ],
    routes: [
      ['Reviews manuelles', '/docs/manual-reviews', 'Demander une PR GitHub ou une MR GitLab via la même file bornée que les webhooks.'],
      ['Politique webhook', '/docs/webhook-policy', 'Contrôler les reviews automatiques par mode, labels, projets, repos et branches.'],
      ['Runtime Docker', '/docs/docker', 'Exécuter l’agent avec Headroom et MemPalace sur le réseau Compose privé.'],
      ['Diagnostic', '/docs/troubleshooting', 'Diagnostiquer pression de file, tokens, readiness sidecar et publication.'],
    ],
  },
};

const commands = [
  ['setup', 'go run ./cmd/7review setup'],
  ['manual github', '7review review github --repo owner/repo --pr 7 --server http://localhost:8080'],
  ['manual gitlab', '7review review gitlab --project 25 --mr 19 --server http://localhost:8080'],
  ['readiness', '7review status --server http://localhost:8080'],
];

function ConsolePanel({content}) {
  return (
    <div className={styles.console} aria-label="7review operator console commands">
      <div className={styles.consoleHeader}>
        <span>7review</span>
        <span>{content.console}</span>
      </div>
      <div className={styles.consoleBody}>
        {commands.map(([label, command]) => (
          <div className={styles.commandRow} key={label}>
            <span className={styles.commandLabel}>{label}</span>
            <div className={styles.commandLine}>
              <span>$</span>
              <code>{command}</code>
            </div>
          </div>
        ))}
      </div>
      <div className={styles.consoleFooter}>
        <span>WEBHOOK_REVIEW_MODE=manual_first</span>
        <span>{content.footer}</span>
      </div>
    </div>
  );
}

function MascotPanel() {
  const mascot = useBaseUrl('/img/7review-mascot.png');
  return (
    <div className={styles.mascotPanel}>
      <img src={mascot} alt="7review mascot with review console" />
      <div className={styles.mascotCaption}>
        <span>7review</span>
        <span>code review agent</span>
      </div>
    </div>
  );
}

export default function Home() {
  const {i18n} = useDocusaurusContext();
  const content = copy[i18n.currentLocale] ?? copy.en;
  return (
    <Layout
      title={content.title}
      description={content.description}>
      <main className={styles.page}>
        <section className={styles.hero}>
          <div className={styles.heroText}>
            <p className={styles.eyebrow}>{content.eyebrow}</p>
            <h1>{content.heading}</h1>
            <p className={styles.lede}>
              {content.lede}
            </p>
            <div className={styles.actions}>
              <Link className={styles.primaryAction} to="/docs/quick-start">{content.start}</Link>
              <Link className={styles.secondaryAction} to="/docs/architecture">{content.architecture}</Link>
            </div>
            <div className={styles.checks} aria-label="Core operating constraints">
              {content.checks.map((item) => <span key={item}>{item}</span>)}
            </div>
          </div>
          <MascotPanel />
        </section>

        <section className={styles.agentSection} aria-label={content.loopTitle}>
          <div className={styles.agentIntro}>
            <span>7review agent</span>
            <h2>{content.loopTitle}</h2>
            <p>{content.loopIntro}</p>
          </div>
          <div className={styles.loopGrid}>
            {content.loop.map(([number, title, body]) => (
              <div className={styles.loopStep} key={number}>
                <span>{number}</span>
                <h3>{title}</h3>
                <p>{body}</p>
              </div>
            ))}
          </div>
        </section>

        <section className={styles.operatorSection} aria-label={content.console}>
          <div className={styles.sectionHeader}>
            <span>7review</span>
            <h2>{content.console}</h2>
          </div>
          <ConsolePanel content={content} />
        </section>

        <section className={styles.routeGrid} aria-label="Documentation paths">
          {content.routes.map(([title, href, body]) => (
            <Link className={styles.routeCard} to={href} key={title}>
              <span>{title}</span>
              <p>{body}</p>
            </Link>
          ))}
        </section>
      </main>
    </Layout>
  );
}
