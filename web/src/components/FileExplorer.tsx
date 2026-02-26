import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { FileText } from 'lucide-react'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { githubGist } from 'react-syntax-highlighter/dist/esm/styles/hljs'
import { api, formatBytes } from '#/api'
import type { SkillFile } from '#/api'
import { MarkdownContent } from './MarkdownContent'
import { SkeletonLines } from './SkeletonLines'

const LANG_MAP: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  py: 'python', rb: 'ruby', go: 'go', rs: 'rust', java: 'java',
  sh: 'bash', zsh: 'bash', bash: 'bash', fish: 'bash',
  json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'toml',
  html: 'html', css: 'css', scss: 'scss', less: 'less',
  xml: 'xml', sql: 'sql', graphql: 'graphql',
  md: 'markdown', mdx: 'markdown',
  c: 'c', cpp: 'cpp', h: 'c', hpp: 'cpp',
  cs: 'csharp', swift: 'swift', kt: 'kotlin',
  dockerfile: 'dockerfile', makefile: 'makefile',
}

function getLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANG_MAP[ext] ?? 'plaintext'
}

export function FileExplorer({
  slug,
  version,
  files,
}: {
  slug: string
  version: string
  files: SkillFile[]
}) {
  const [selectedFile, setSelectedFile] = useState<string | null>(null)

  useEffect(() => {
    if (!selectedFile && files.length > 0) {
      const nonSkillMd = files.find(
        (f) => f.path !== 'SKILL.md' && f.path !== 'skills.md',
      )
      setSelectedFile(nonSkillMd ? nonSkillMd.path : files[0].path)
    }
  }, [files, selectedFile])

  const { data: fileContent, isLoading: loadingContent } = useQuery({
    queryKey: ['file-content', slug, selectedFile, version],
    queryFn: () => api.getFileContent(slug, selectedFile!, version),
    enabled: !!selectedFile,
  })

  const [fileHash, setFileHash] = useState<string | null>(null)
  useEffect(() => {
    if (!fileContent) {
      setFileHash(null)
      return
    }
    const bytes = new TextEncoder().encode(fileContent)
    crypto.subtle.digest('SHA-256', bytes).then((buf) => {
      const hex = Array.from(new Uint8Array(buf))
        .map((b) => b.toString(16).padStart(2, '0'))
        .join('')
      setFileHash(hex)
    })
  }, [fileContent])

  return (
    <div
      className="relative border border-base-300 rounded-lg"
      style={{ height: 480 }}
    >
      {/* File list */}
      <div
        className="absolute top-0 left-0 bottom-0 border-r border-base-300"
        style={{ width: '40%' }}
      >
        <div className="flex justify-between items-center px-3 py-2 border-b border-base-300 bg-base-200">
          <span className="text-xs font-medium text-base-content/50">
            Files
          </span>
          <span className="text-xs text-base-content/40">
            {files.length} total
          </span>
        </div>
        <div
          className="absolute left-0 right-0 bottom-0 overflow-y-auto"
          style={{ top: 37 }}
        >
          {files.map((file, idx) => (
            <button
              key={file.path}
              onClick={() => setSelectedFile(file.path)}
              className={`w-full flex justify-between items-center px-3 py-2 text-left transition-colors ${
                idx !== files.length - 1 ? 'border-b border-base-300' : ''
              } ${selectedFile === file.path ? 'bg-base-200' : 'hover:bg-base-100'}`}
            >
              <span className="truncate font-mono text-xs text-base-content/80">
                {file.path}
              </span>
              <span className="text-xs text-base-content/40 ml-2 shrink-0">
                {formatBytes(file.size)}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* File preview */}
      <div
        className="absolute top-0 right-0 bottom-0"
        style={{ left: '40%' }}
      >
        {/* Info bar */}
        <div className="flex items-center justify-between gap-2 px-3 py-1.5 border-b border-base-300 bg-base-200">
          <span className="font-mono text-xs text-base-content/60 truncate">
            {selectedFile ?? '—'}
          </span>
          {fileHash && (
            <span
              className="font-mono text-[10px] text-base-content/30 shrink-0 tabular-nums"
              title={`SHA-256: ${fileHash}`}
            >
              {fileHash.slice(0, 7)}
            </span>
          )}
        </div>

        {/* Scrollable content */}
        <div
          className="absolute left-0 right-0 bottom-0 overflow-auto"
          style={{ top: 33 }}
        >
          {selectedFile ? (
            loadingContent ? (
              <div className="p-5">
                <SkeletonLines lines={8} height="h-3" />
              </div>
            ) : fileContent ? (
              selectedFile.endsWith('.md') ? (
                <div className="prose prose-sm max-w-none dark:prose-invert p-5">
                  <MarkdownContent
                    content={fileContent}
                    stripHeading={/^skill\.md$/i.test(selectedFile)}
                  />
                </div>
              ) : (
                <SyntaxHighlighter
                  language={getLanguage(selectedFile)}
                  style={githubGist}
                  customStyle={{
                    margin: 0,
                    padding: '20px',
                    fontSize: '12px',
                    lineHeight: '1.6',
                    background: 'transparent',
                  }}
                  showLineNumbers
                  lineNumberStyle={{ color: '#bbb', minWidth: '2.5em' }}
                  wrapLongLines
                >
                  {fileContent}
                </SyntaxHighlighter>
              )
            ) : (
              <div className="flex items-center justify-center h-full text-base-content/40 text-sm">
                No content to display
              </div>
            )
          ) : (
            <div className="flex flex-col items-center justify-center h-full gap-2 text-base-content/30">
              <FileText className="h-8 w-8" />
              <p className="text-sm font-medium">Select a file</p>
              <p className="text-xs">Select a file to preview.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
