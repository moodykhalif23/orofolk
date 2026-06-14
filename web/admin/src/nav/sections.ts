export interface NavLeaf {
  label: string
  icon: string
  /** Vue Router route name the leaf links to. */
  routeName: string
  /** One-line description shown on the section hub card. */
  blurb: string
  /** RBAC permission required; absent = always visible (server still enforces). */
  permission?: string
  /** Plan feature gating the leaf; hidden when the org's plan lacks it. */
  feature?: string
}

export interface NavSection {
  /** Stable key — also the hub route param (/section/:key). */
  key: string
  label: string
  icon: string
  /** Headline shown on the hub page. */
  blurb: string
  /** Standalone sections link straight here and show no sub-nav (Dashboard, Assistant). */
  routeName?: string
  /** Plan feature gating a standalone section. */
  feature?: string
  items?: NavLeaf[]
}

export const sections: NavSection[] = [
  {
    key: 'dashboard',
    label: 'Dashboard',
    icon: 'pi pi-home',
    blurb: 'Your business at a glance.',
    routeName: 'dashboard',
  },
  {
    key: 'catalog',
    label: 'Catalog',
    icon: 'pi pi-th-large',
    blurb: 'Everything you sell — products, structure and how buyers find them.',
    items: [
      { label: 'Products', icon: 'pi pi-box', routeName: 'products', permission: 'product.view', blurb: 'Create and manage sellable items and variants.' },
      { label: 'Categories', icon: 'pi pi-sitemap', routeName: 'categories', permission: 'category.view', blurb: 'Organize the catalog into a browsable tree.' },
      { label: 'Attributes', icon: 'pi pi-tags', routeName: 'attributes', permission: 'attribute.view', blurb: 'Define the specs and options products share.' },
      { label: 'Configurator', icon: 'pi pi-sliders-h', routeName: 'configurator', permission: 'product.view', blurb: 'Build guided, rule-based product configurations.' },
      { label: 'Search merchandising', icon: 'pi pi-search-plus', routeName: 'merchandising', permission: 'merchandising.view', feature: 'merchandising', blurb: 'Tune search results, synonyms and boosts.' },
    ],
  },
  {
    key: 'pricing',
    label: 'Pricing',
    icon: 'pi pi-dollar',
    blurb: 'How every customer sees their price.',
    items: [
      { label: 'Price lists', icon: 'pi pi-dollar', routeName: 'pricing', permission: 'price_list.view', blurb: 'Per-customer and contract price books.' },
      { label: 'Price rules', icon: 'pi pi-sliders-h', routeName: 'price-rules', permission: 'price_list.view', blurb: 'Conditional pricing, tiers and volume breaks.' },
      { label: 'Promotions', icon: 'pi pi-tag', routeName: 'promotions', permission: 'promotion.view', blurb: 'Time-boxed discounts and campaign offers.' },
      { label: 'Exchange rates', icon: 'pi pi-money-bill', routeName: 'fx-rates', permission: 'fx.view', feature: 'fx', blurb: 'Multi-currency FX rates for pricing and invoicing.' },
      { label: 'Tax & shipping', icon: 'pi pi-percentage', routeName: 'tax-shipping', permission: 'tax.view', blurb: 'Tax classes and shipping methods and rates.' },
    ],
  },
  {
    key: 'sales',
    label: 'Sales',
    icon: 'pi pi-shopping-cart',
    blurb: 'From request to cash — the full order-to-cash motion.',
    items: [
      { label: 'RFQs', icon: 'pi pi-inbox', routeName: 'rfqs', permission: 'rfq.view', blurb: 'Inbound requests for quote from buyers.' },
      { label: 'Quotes', icon: 'pi pi-file-edit', routeName: 'quotes', permission: 'quote.view', blurb: 'Negotiate and issue priced quotes.' },
      { label: 'Orders', icon: 'pi pi-shopping-cart', routeName: 'orders', permission: 'order.view', blurb: 'Manage orders through fulfilment.' },
      { label: 'Subscriptions', icon: 'pi pi-sync', routeName: 'subscriptions', permission: 'subscription.view', feature: 'subscriptions', blurb: 'Recurring orders and billing schedules.' },
      { label: 'Rebates', icon: 'pi pi-percentage', routeName: 'rebates', permission: 'rebate.view', feature: 'rebates', blurb: 'Accrue and settle customer rebates.' },
      { label: 'Invoices', icon: 'pi pi-receipt', routeName: 'invoices', permission: 'invoice.view', blurb: 'Issue invoices and track payment.' },
      { label: 'AR aging', icon: 'pi pi-chart-line', routeName: 'ar-aging', permission: 'invoice.view', blurb: 'See who owes what, by age bucket.' },
      { label: 'Returns', icon: 'pi pi-replay', routeName: 'returns', permission: 'return.view', blurb: 'Process returns, RMAs and credits.' },
    ],
  },
  {
    key: 'customers',
    label: 'Customers',
    icon: 'pi pi-users',
    blurb: 'Your accounts, contacts and the health of each relationship.',
    items: [
      { label: 'Customers', icon: 'pi pi-building', routeName: 'customers', permission: 'customer.view', blurb: 'Company accounts, contacts and credit.' },
      { label: 'Customer groups', icon: 'pi pi-users', routeName: 'customer-groups', permission: 'customer.view', blurb: 'Segment accounts for pricing and rules.' },
      { label: 'Leads', icon: 'pi pi-filter', routeName: 'leads', permission: 'crm.view', blurb: 'Inbound and prospected leads.' },
      { label: 'Pipeline', icon: 'pi pi-chart-bar', routeName: 'pipeline', permission: 'crm.view', blurb: 'Track deals across your sales stages.' },
      { label: 'Opportunities', icon: 'pi pi-briefcase', routeName: 'opportunities', permission: 'crm.view', blurb: 'Open deals with value and forecast.' },
      { label: 'Account health', icon: 'pi pi-heart', routeName: 'account-health', permission: 'crm.view', blurb: 'Spot churn risk and at-risk accounts.' },
    ],
  },
  {
    key: 'marketplace',
    label: 'Marketplace',
    icon: 'pi pi-shop',
    blurb: 'Run a multi-vendor marketplace.',
    items: [
      { label: 'Vendors', icon: 'pi pi-shop', routeName: 'vendors', permission: 'vendor.view', blurb: 'Onboard and manage selling partners.' },
      { label: 'Catalog moderation', icon: 'pi pi-check-square', routeName: 'moderation', permission: 'vendor.view', blurb: 'Review and approve vendor listings.' },
    ],
  },
  {
    key: 'operations',
    label: 'Operations',
    icon: 'pi pi-warehouse',
    blurb: 'Stock and field operations.',
    items: [
      { label: 'Inventory', icon: 'pi pi-warehouse', routeName: 'inventory', permission: 'inventory.view', blurb: 'Stock levels across warehouses.' },
      { label: 'Field devices', icon: 'pi pi-mobile', routeName: 'field-devices', permission: 'field.sync', blurb: 'Register and sync field and edge devices.' },
    ],
  },
  {
    key: 'automation',
    label: 'Automation',
    icon: 'pi pi-bolt',
    blurb: 'Let the system do the repetitive work.',
    items: [
      { label: 'Workflows', icon: 'pi pi-sitemap', routeName: 'workflows', permission: 'workflow.view', blurb: 'Visual, multi-step process automations.' },
      { label: 'Automation rules', icon: 'pi pi-bolt', routeName: 'automation-rules', permission: 'workflow.view', blurb: 'Event-triggered if-this-then-that rules.' },
      { label: 'Approval routing', icon: 'pi pi-check-circle', routeName: 'approval-routing', permission: 'workflow.view', blurb: 'Route approvals by amount and role.' },
    ],
  },
  {
    key: 'content',
    label: 'Content',
    icon: 'pi pi-folder',
    blurb: 'Storefront content and media.',
    items: [
      { label: 'Pages', icon: 'pi pi-file', routeName: 'pages', permission: 'cms.view', blurb: 'Build and publish storefront pages.' },
      { label: 'Media', icon: 'pi pi-images', routeName: 'media', permission: 'cms.view', blurb: 'Your digital asset library.' },
    ],
  },
  {
    key: 'insights',
    label: 'Insights',
    icon: 'pi pi-chart-bar',
    blurb: 'Understand the business — then export the data.',
    items: [
      { label: 'Briefings', icon: 'pi pi-sparkles', routeName: 'insights', permission: 'report.view', blurb: 'AI-authored weekly executive briefings.' },
      { label: 'Analytics', icon: 'pi pi-chart-line', routeName: 'analytics', permission: 'report.view', blurb: 'Live dashboards across the business.' },
      { label: 'Report builder', icon: 'pi pi-table', routeName: 'report-builder', permission: 'report.view', blurb: 'Compose safe, custom reports.' },
      { label: 'Data export', icon: 'pi pi-download', routeName: 'exports', permission: 'report.view', blurb: 'Full-record CSV and Excel exports.' },
    ],
  },
  {
    key: 'settings',
    label: 'Settings',
    icon: 'pi pi-cog',
    blurb: 'Configure the platform, billing and integrations.',
    items: [
      { label: 'Platform', icon: 'pi pi-building-columns', routeName: 'platform-orgs', permission: 'platform.view', blurb: 'Manage organizations (platform admin).' },
      { label: 'Billing & usage', icon: 'pi pi-credit-card', routeName: 'billing', blurb: 'Plan, usage and invoices.' },
      { label: 'Websites', icon: 'pi pi-globe', routeName: 'websites', permission: 'tenant.view', blurb: 'Storefronts and domains.' },
      { label: 'Configuration', icon: 'pi pi-cog', routeName: 'settings', permission: 'settings.view', blurb: 'Store identity and core settings.' },
      { label: 'Audit log', icon: 'pi pi-shield', routeName: 'audit-log', permission: 'audit.view', blurb: 'Every staff action, recorded.' },
      { label: 'Integrations', icon: 'pi pi-sync', routeName: 'integrations', permission: 'integration.view', blurb: 'Connect external systems.' },
      { label: 'ERP sync', icon: 'pi pi-server', routeName: 'erp', permission: 'erp.view', blurb: 'Sync orders and inventory with your ERP.' },
      { label: 'SSO providers', icon: 'pi pi-id-card', routeName: 'identity-providers', permission: 'sso.view', blurb: 'Configure single sign-on.' },
    ],
  },
  {
    key: 'assistant',
    label: 'Assistant',
    icon: 'pi pi-sparkles',
    blurb: 'Ask across orders, catalog, customers and stock.',
    routeName: 'assistant',
    feature: 'assistant',
  },
]

type Can = (permission: string) => boolean
type Allows = (feature: string) => boolean

/** Section reduced to the leaves the current user/plan can actually see. */
export function visibleLeaves(section: NavSection, can: Can, allows: Allows): NavLeaf[] {
  return (section.items ?? []).filter(
    (i) => (!i.permission || can(i.permission)) && (!i.feature || allows(i.feature)),
  )
}

/** True when a section should appear in the head nav for this user/plan. */
export function isSectionVisible(section: NavSection, can: Can, allows: Allows): boolean {
  if (section.feature && !allows(section.feature)) return false
  if (section.routeName) return true // standalone (Dashboard, Assistant)
  return visibleLeaves(section, can, allows).length > 0
}

export function sectionByKey(key: string): NavSection | undefined {
  return sections.find((s) => s.key === key)
}
