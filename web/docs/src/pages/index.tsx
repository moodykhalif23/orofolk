import type {ReactNode} from 'react'
import clsx from 'clsx'
import Link from '@docusaurus/Link'
import useDocusaurusContext from '@docusaurus/useDocusaurusContext'
import Layout from '@theme/Layout'
import Heading from '@theme/Heading'
import styles from './index.module.css'

type Feature = {
  icon: string
  title: string
  body: ReactNode
}

const FEATURES: Feature[] = [
  {
    icon: '🧩',
    title: 'Commerce + CRM + workflow',
    body: 'Catalog, pricing, quotes, orders, invoicing, accounts and a low-code automation engine — one system, not a stack of integrations.',
  },
  {
    icon: '🔌',
    title: 'API-first by contract',
    body: (
      <>
        A single <strong>OpenAPI 3.1</strong> document is the source of truth. The
        typed client and this reference are generated from it, so they never drift.
      </>
    ),
  },
  {
    icon: '🎨',
    title: 'Drag-and-drop storefront',
    body: 'Build pages from blocks with a live, true-to-production preview — or let AI draft a layout you can tweak and publish.',
  },
  {
    icon: '⚡',
    title: 'Visual automation',
    body: 'Design rules on a flow canvas: when an event fires and your conditions match, actions run as background jobs.',
  },
  {
    icon: '🤖',
    title: 'Org-aware assistant',
    body: 'Ask about orders, invoices, catalog, customers and stock. Every answer runs under the caller’s permissions and tenant scope.',
  },
  {
    icon: '📦',
    title: 'Self-hosted monolith',
    body: (
      <>
        A clean Go modular monolith on PostgreSQL 16, shipped with Docker Compose.
        Your data, your infrastructure.
      </>
    ),
  },
]

const STACK = ['Go', 'PostgreSQL 16', 'Vue 3', 'Nuxt', 'OpenAPI 3.1', 'Docker']

function Hero(): ReactNode {
  return (
    <header className={styles.hero}>
      <div className={clsx('container', styles.heroInner)}>
        <span className={styles.badge}>Self-hosted · API-first</span>
        <Heading as="h1" className={styles.heroTitle}>
          B2B commerce, CRM &amp; workflow automation — in one platform.
        </Heading>
        <p className={styles.heroSubtitle}>
          Teggo is a self-hosted commerce platform for manufacturers, distributors
          and wholesalers. A Go modular monolith with a single OpenAPI contract that
          powers a typed client, a Vue admin, and a Nuxt storefront.
        </p>
        <div className={styles.ctaRow}>
          <Link className={clsx('button button--primary button--lg', styles.cta)} to="/getting-started">
            Get started
          </Link>
          <Link className={clsx('button button--secondary button--lg', styles.cta)} to="/api">
            API reference
          </Link>
        </div>
        <div className={styles.terminal} aria-label="Quickstart commands">
          <div className={styles.terminalBar}>
            <span /><span /><span />
          </div>
          <pre className={styles.terminalBody}>
            <code>
              <span className={styles.prompt}>$</span> git clone https://github.com/teggo/teggo{'\n'}
              <span className={styles.prompt}>$</span> make up   <span className={styles.comment}># Postgres + API + admin + storefront</span>{'\n'}
              <span className={styles.prompt}>$</span> open http://localhost:8080
            </code>
          </pre>
        </div>
        <ul className={styles.stack} aria-label="Built with">
          {STACK.map((s) => (
            <li key={s} className={styles.stackItem}>{s}</li>
          ))}
        </ul>
      </div>
    </header>
  )
}

function Features(): ReactNode {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.grid}>
          {FEATURES.map((f) => (
            <div key={f.title} className={styles.card}>
              <div className={styles.cardIcon} aria-hidden="true">{f.icon}</div>
              <Heading as="h3" className={styles.cardTitle}>{f.title}</Heading>
              <p className={styles.cardBody}>{f.body}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function Contract(): ReactNode {
  return (
    <section className={clsx(styles.section, styles.contract)}>
      <div className={clsx('container', styles.contractInner)}>
        <div className={styles.contractCopy}>
          <Heading as="h2" className={styles.contractTitle}>One contract, zero drift</Heading>
          <p className={styles.contractText}>
            The Go service is the single source of truth. From one OpenAPI 3.1
            document we generate the TypeScript client every frontend consumes —
            and the API reference you’re reading. Change the contract, regenerate,
            and everything stays in lockstep.
          </p>
          <Link className="button button--primary button--lg" to="/api">
            Browse the API reference
          </Link>
        </div>
        <div className={styles.flowCard}>
          <span className={styles.flowStep}>OpenAPI 3.1 contract</span>
          <span className={styles.flowArrow}>↓</span>
          <span className={styles.flowStep}>Generated typed client</span>
          <span className={styles.flowArrow}>↓</span>
          <span className={styles.flowStep}>Vue admin · Nuxt storefront · API docs</span>
        </div>
      </div>
    </section>
  )
}

function FinalCta(): ReactNode {
  return (
    <section className={styles.finalCta}>
      <div className={clsx('container', styles.finalInner)}>
        <Heading as="h2" className={styles.finalTitle}>Run the whole stack locally in minutes.</Heading>
        <div className={styles.ctaRow}>
          <Link className="button button--primary button--lg" to="/getting-started">
            Get started
          </Link>
          <Link className="button button--outline button--lg" to="/intro">
            Read the docs
          </Link>
        </div>
      </div>
    </section>
  )
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext()
  return (
    <Layout
      title="Teggo — self-hosted B2B commerce"
      description={siteConfig.tagline}
    >
      <Hero />
      <main>
        <Features />
        <Contract />
        <FinalCta />
      </main>
    </Layout>
  )
}
