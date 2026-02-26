import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import SyntaxHighlighter from 'react-syntax-highlighter'
import { githubGist, vs2015 } from 'react-syntax-highlighter/dist/esm/styles/hljs'
import { useTheme } from '#/hooks/useTheme'

function stripFrontMatter(content: string): string {
  return content.replace(/^---[\s\S]*?---\n?/, '').trimStart()
}

export function MarkdownContent({
  content,
  stripHeading = false,
}: {
  content: string
  stripHeading?: boolean
}) {
  const { isDark: dark } = useTheme()
  const rendered = stripHeading ? stripFrontMatter(content) : content
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        h1: ({ node, ...props }) => (
          <h1 className="text-2xl font-bold mt-4 mb-2" {...props} />
        ),
        h2: ({ node, ...props }) => (
          <h2 className="text-xl font-bold mt-3 mb-2" {...props} />
        ),
        h3: ({ node, ...props }) => (
          <h3 className="text-lg font-bold mt-2 mb-1" {...props} />
        ),
        p: ({ node, ...props }) => (
          <p className="my-2 leading-relaxed" {...props} />
        ),
        ul: ({ node, ...props }) => (
          <ul className="list-disc list-inside my-2 space-y-1" {...props} />
        ),
        ol: ({ node, ...props }) => (
          <ol className="list-decimal list-inside my-2 space-y-1" {...props} />
        ),
        li: ({ node, ...props }) => <li className="ml-2" {...props} />,
        code: ({ node, className, children, ...props }) => {
          const lang = className?.replace('language-', '') ?? ''
          if (!lang) {
            return (
              <code
                className="bg-base-300 px-1.5 py-0.5 rounded text-xs font-mono"
                {...props}
              >
                {children}
              </code>
            )
          }
          return (
            <SyntaxHighlighter
              language={lang}
              style={dark ? vs2015 : githubGist}
              customStyle={{
                margin: '8px 0',
                borderRadius: '6px',
                fontSize: '12px',
              }}
              PreTag="div"
            >
              {String(children).replace(/\n$/, '')}
            </SyntaxHighlighter>
          )
        },
        pre: ({ node, children, ...props }) => (
          <div {...(props as React.HTMLAttributes<HTMLDivElement>)}>
            {children}
          </div>
        ),
        blockquote: ({ node, ...props }) => (
          <blockquote
            className="border-l-4 border-primary pl-4 italic my-2 text-base-content/70"
            {...props}
          />
        ),
        a: ({ node, ...props }) => (
          <a
            className="text-primary hover:underline"
            target="_blank"
            rel="noopener noreferrer"
            {...props}
          />
        ),
        table: ({ node, ...props }) => (
          <div className="overflow-x-auto my-3">
            <table
              className="table table-sm table-zebra w-full border border-base-300 rounded"
              {...props}
            />
          </div>
        ),
        thead: ({ node, ...props }) => (
          <thead className="bg-base-200" {...props} />
        ),
        th: ({ node, ...props }) => (
          <th
            className="border border-base-300 px-3 py-1.5 text-left text-xs font-semibold"
            {...props}
          />
        ),
        td: ({ node, ...props }) => (
          <td className="border border-base-300 px-3 py-1.5 text-xs" {...props} />
        ),
      }}
    >
      {rendered}
    </ReactMarkdown>
  )
}
