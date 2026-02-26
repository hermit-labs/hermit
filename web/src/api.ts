const API_BASE = '/api/v1'
const INTERNAL_BASE = '/api/internal'

export class APIError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'APIError'
  }
}

function authHeaders(): HeadersInit {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: {
      ...authHeaders(),
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })

  if (!res.ok) {
    let msg = res.statusText
    try {
      const body = await res.json()
      msg = body.message ?? msg
    } catch {
      // ignore
    }
    throw new APIError(res.status, msg)
  }

  return res.json() as Promise<T>
}

// Token management
export function getToken(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem('hermit_token')
}

export function setToken(token: string): void {
  localStorage.setItem('hermit_token', token)
}

export function clearToken(): void {
  localStorage.removeItem('hermit_token')
}

// Types
export interface SkillStats {
  downloads: number
  stars: number
  installsCurrent: number
  installsAllTime: number
}

export interface SkillVersionSummary {
  version: string
  createdAt: number
  changelog: string
}

export interface SkillFile {
  path: string
  size: number
}

export interface SkillVersionDetail extends SkillVersionSummary {
  changelogSource: string | null
  files?: SkillFile[]
}

export interface SkillSummary {
  slug: string
  displayName: string
  summary: string | null
  tags: Record<string, string>
  stats: SkillStats
  createdAt: number
  updatedAt: number
  latestVersion?: SkillVersionSummary
}

export interface SkillDetail extends SkillSummary {
  latestVersion?: SkillVersionDetail
}

export interface User {
  handle: string
  displayName: string
  image: string | null
}

export interface AdminToken {
  subject: string
  token: string
  id: string
  name: string
}

export interface PersonalAccessToken {
  id: string
  subject: string
  name: string
  token_type: string
  is_admin: boolean
  disabled: boolean
  created_at: string
  last_used_at: string | null
}

export interface CreateTokenResult {
  token: string
  id: string
  name: string
}

export interface SearchResult extends SkillSummary {
  score: number
}

export interface RepoStatsView {
  id: string
  name: string
  type: string
  upstreamUrl: string | null
  enabled: boolean
  skillCount: number
}

export interface DashboardStats {
  totalSkills: number
  totalVersions: number
  totalDownloads: number
  totalStars: number
  totalInstalls: number
  repositories: RepoStatsView[]
}

export interface SyncSource {
  id: string
  name: string
  upstreamUrl: string
  enabled: boolean
  skillCount: number
}

export interface SyncResult {
  Repositories: number
  Skills: number
  Versions: number
  Cached: number
  Failed: number
}

export interface ProxySyncConfig {
  page_size: number
  concurrency: number
}

export interface SyncStatusResponse {
  configured: boolean
  running: boolean
  lastResult: SyncResult | null
  lastError: string
}

export interface RepoMember {
  repoId: string
  repoName: string
  subject: string
  role: 'admin' | 'developer' | 'viewer'
}

// API functions
export const api = {
  listSkills: (params?: { limit?: number; cursor?: string; sort?: string }) => {
    const q = new URLSearchParams()
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.cursor) q.set('cursor', params.cursor)
    if (params?.sort) q.set('sort', params.sort)
    return request<{ items: SkillSummary[]; nextCursor: string | null }>(
      `${API_BASE}/skills?${q}`,
    )
  },

  searchSkills: (query: string) =>
    request<{ results: SearchResult[] }>(
      `${API_BASE}/search?q=${encodeURIComponent(query)}`,
    ),

  getSkill: (slug: string) =>
    request<{
      skill: SkillSummary
      latestVersion: SkillVersionDetail | null
      owner: User | null
    }>(`${API_BASE}/skills/${slug}`),

  listVersions: (
    slug: string,
    params?: { limit?: number; cursor?: string },
  ) => {
    const q = new URLSearchParams()
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.cursor) q.set('cursor', params.cursor)
    return request<{
      items: SkillVersionSummary[]
      nextCursor: string | null
    }>(`${API_BASE}/skills/${slug}/versions?${q}`)
  },

  getVersion: (slug: string, version: string) =>
    request<{
      version: SkillVersionDetail
      skill: { slug: string; displayName: string }
    }>(`${API_BASE}/skills/${slug}/versions/${version}`),

  whoami: () => request<{ user: User }>(`${API_BASE}/whoami`),

  publishSkill: async (
    payload: {
      slug: string
      displayName: string
      version: string
      changelog: string
      summary?: string
      tags?: string[]
    },
    files: File[],
  ) => {
    const token = getToken()
    const formData = new FormData()
    formData.append('payload', JSON.stringify(payload))
    for (const file of files) {
      formData.append('files', file)
    }

    const res = await fetch(`${API_BASE}/skills`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: formData,
    })

    if (!res.ok) {
      let msg = res.statusText
      try {
        const body = await res.json()
        msg = body.message ?? msg
      } catch {
        // ignore
      }
      throw new APIError(res.status, msg)
    }

    return res.json() as Promise<{
      ok: boolean
      skillId: string
      versionId: string
    }>
  },

  deleteSkill: (slug: string) =>
    request<Record<string, never>>(`${API_BASE}/skills/${slug}`, {
      method: 'DELETE',
    }),

  undeleteSkill: (slug: string) =>
    request<Record<string, never>>(`${API_BASE}/skills/${slug}/undelete`, {
      method: 'POST',
    }),

  // Admin: create token for any user
  adminCreateToken: (subject: string, name: string) =>
    request<AdminToken>(`${INTERNAL_BASE}/tokens`, {
      method: 'POST',
      body: JSON.stringify({ subject, name }),
    }),

  // User self-service: Personal Access Tokens
  listMyTokens: () =>
    request<{ tokens: PersonalAccessToken[] }>(`${API_BASE}/tokens`),

  createMyToken: (name: string) =>
    request<CreateTokenResult>(`${API_BASE}/tokens`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  revokeMyToken: (tokenId: string) =>
    request<{ ok: boolean }>(`${API_BASE}/tokens/${tokenId}`, {
      method: 'DELETE',
    }),

  getDashboardStats: () => request<DashboardStats>(`${INTERNAL_BASE}/stats`),

  listSyncSources: () =>
    request<{ sources: SyncSource[] }>(`${INTERNAL_BASE}/sync-sources`),

  addSyncSource: (name: string, upstreamUrl: string) =>
    request<SyncSource>(`${INTERNAL_BASE}/sync-sources`, {
      method: 'POST',
      body: JSON.stringify({ name, upstreamUrl }),
    }),

  removeSyncSource: (id: string) =>
    request<{ ok: boolean }>(`${INTERNAL_BASE}/sync-sources/${id}`, {
      method: 'DELETE',
    }),

  toggleSyncSource: (id: string, enabled: boolean) =>
    request<{ ok: boolean }>(`${INTERNAL_BASE}/sync-sources/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    }),

  triggerSync: () =>
    request<{ ok: boolean; message: string }>(`${INTERNAL_BASE}/sync`, {
      method: 'POST',
    }),

  getSyncStatus: () =>
    request<SyncStatusResponse>(`${INTERNAL_BASE}/sync/status`),

  getProxySyncConfig: () =>
    request<ProxySyncConfig>(`${INTERNAL_BASE}/sync/config`),

  saveProxySyncConfig: (config: ProxySyncConfig) =>
    request<{ ok: boolean }>(`${INTERNAL_BASE}/sync/config`, {
      method: 'PUT',
      body: JSON.stringify(config),
    }),

  // RBAC
  listAllMembers: () =>
    request<{ members: RepoMember[] }>(`${INTERNAL_BASE}/rbac/members`),

  listRepoMembers: (repoId: string) =>
    request<{ members: RepoMember[] }>(
      `${INTERNAL_BASE}/rbac/repos/${repoId}/members`,
    ),

  assignMember: (repoId: string, subject: string, role: string) =>
    request<{ ok: boolean }>(
      `${INTERNAL_BASE}/rbac/repos/${repoId}/members`,
      {
        method: 'POST',
        body: JSON.stringify({ subject, role }),
      },
    ),

  removeMember: (repoId: string, subject: string) =>
    request<{ ok: boolean }>(
      `${INTERNAL_BASE}/rbac/repos/${repoId}/members/${encodeURIComponent(subject)}`,
      { method: 'DELETE' },
    ),

  downloadUrl: (
    slug: string,
    opts?: { version?: string; tag?: string },
  ): string => {
    const q = new URLSearchParams({ slug })
    if (opts?.version) q.set('version', opts.version)
    if (opts?.tag) q.set('tag', opts.tag)
    return `${API_BASE}/download?${q}`
  },

  fileUrl: (slug: string, path: string, version?: string): string => {
    const q = new URLSearchParams({ path })
    if (version) q.set('version', version)
    return `${API_BASE}/skills/${slug}/file?${q}`
  },

  getFileContent: (slug: string, path: string, version?: string) =>
    fetch(
      `${API_BASE}/skills/${slug}/file?${new URLSearchParams({ path, ...(version && { version }) })}`,
      {
        headers: authHeaders(),
      },
    ).then((res) => {
      if (!res.ok) throw new Error('Failed to load file')
      return res.text()
    }),
}

// Auth Config (LDAP) management
export interface AuthConfigItem {
  provider_type: string
  enabled: boolean
  config: Record<string, any>
  updated_at: string
}

export const authConfigApi = {
  list: () =>
    request<{ configs: AuthConfigItem[] }>(`${INTERNAL_BASE}/auth-configs`),

  get: (providerType: string) =>
    request<AuthConfigItem>(`${INTERNAL_BASE}/auth-configs/${providerType}`),

  save: (providerType: string, enabled: boolean, config: Record<string, any>) =>
    request<{ ok: boolean }>(`${INTERNAL_BASE}/auth-configs/${providerType}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled, config }),
    }),

  remove: (providerType: string) =>
    request<{ ok: boolean }>(`${INTERNAL_BASE}/auth-configs/${providerType}`, {
      method: 'DELETE',
    }),
}

// Auth provider types
export interface AuthProvider {
  id: string
  name: string
  type: 'standard' | 'ldap'
}

export interface LoginResult {
  token: string
  subject: string
  display_name: string
  email: string
  is_admin: boolean
}

export const authApi = {
  getProviders: () =>
    request<{ providers: AuthProvider[] }>(`${API_BASE}/auth/providers`),

  login: (username: string, password: string) =>
    request<LoginResult>(`${API_BASE}/auth/login`, {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  ldapLogin: (username: string, password: string) =>
    request<LoginResult>(`${API_BASE}/auth/ldap`, {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

}

// Admin user management
export interface LocalUser {
  id: string
  username: string
  display_name: string
  email: string
  is_admin: boolean
  disabled: boolean
  created_at: string
}

export const userApi = {
  list: () =>
    request<{ users: LocalUser[] }>(`${INTERNAL_BASE}/users`),

  create: (data: {
    username: string
    password: string
    display_name: string
    email: string
    is_admin: boolean
  }) =>
    request<LocalUser>(`${INTERNAL_BASE}/users`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  update: (
    id: string,
    data: {
      display_name: string
      email: string
      is_admin: boolean
      disabled: boolean
    },
  ) =>
    request<LocalUser>(`${INTERNAL_BASE}/users/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  resetPassword: (id: string, password: string) =>
    request<{ ok: boolean }>(
      `${INTERNAL_BASE}/users/${id}/reset-password`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      },
    ),

  remove: (id: string) =>
    request<{ ok: boolean }>(`${INTERNAL_BASE}/users/${id}`, {
      method: 'DELETE',
    }),
}

// Helpers
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function timeAgo(ms: number): string {
  const seconds = Math.floor((Date.now() - ms) / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  const months = Math.floor(days / 30)
  if (months < 12) return `${months}mo ago`
  return `${Math.floor(months / 12)}y ago`
}

export function formatDate(ms: number): string {
  return new Date(ms).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function skillColor(slug: string): string {
  const gradients = [
    'from-violet-600 to-purple-700',
    'from-blue-600 to-cyan-700',
    'from-emerald-600 to-teal-700',
    'from-orange-500 to-amber-600',
    'from-pink-600 to-rose-700',
    'from-indigo-600 to-blue-700',
    'from-red-600 to-orange-700',
    'from-teal-600 to-emerald-700',
  ]
  let hash = 0
  for (let i = 0; i < slug.length; i++) {
    hash = ((hash << 5) - hash + slug.charCodeAt(i)) | 0
  }
  return gradients[Math.abs(hash) % gradients.length]
}
