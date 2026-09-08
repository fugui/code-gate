import React, { useState, useEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Send, Square, Bot, User, Sparkles, ChevronDown, ChevronRight, Zap } from 'lucide-react'
import { fetchModels } from '../../api/client'
import { ModelItem, ChatMessage } from '../../types'

export const ChatPage: React.FC = () => {
  const [models, setModels] = useState<ModelItem[]>([])
  const [selectedModel, setSelectedModel] = useState<string>('deepseek-chat')
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      id: 'welcome',
      role: 'assistant',
      content: '你好！我是 CodeGate AI 统一网关助手。所有模型请求均享受 100% 原始字节直通与智能 KV Cache 亲和性加速，请随时向我提问！',
    },
  ])
  const [input, setInput] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const abortControllerRef = useRef<AbortController | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // 展开折叠思维链状态
  const [expandedThinking, setExpandedThinking] = useState<Record<string, boolean>>({})

  useEffect(() => {
    fetchModels()
      .then((data) => {
        if (data && data.length > 0) {
          setModels(data)
          setSelectedModel(data[0].id)
        }
      })
      .catch(() => {
        // 使用默认模型备选项
        setModels([
          { id: 'deepseek-chat', object: 'model', owned_by: 'deepseek', cost_multiplier: 1.0, input_rate: 1.0, cache_hit_rate: 0.1, output_rate: 5.0 },
          { id: 'qwen-2.5-coder-32b', object: 'model', owned_by: 'qwen', cost_multiplier: 0.8, input_rate: 1.0, cache_hit_rate: 0.1, output_rate: 5.0 },
        ])
      })
  }, [])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const toggleThinking = (id: string) => {
    setExpandedThinking((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  const handleStop = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
      setIsStreaming(false)
    }
  }

  const handleSend = async () => {
    if (!input.trim() || isStreaming) return

    const userText = input.trim()
    setInput('')

    const userMsgId = `user_${Date.now()}`
    const assistantMsgId = `assistant_${Date.now()}`

    const newMessages: ChatMessage[] = [
      ...messages,
      { id: userMsgId, role: 'user', content: userText },
      { id: assistantMsgId, role: 'assistant', content: '', reasoning_content: '', isThinking: true },
    ]
    setMessages(newMessages)
    setIsStreaming(true)

    const controller = new AbortController()
    abortControllerRef.current = controller

    const startTime = Date.now()
    let firstTokenTime = 0
    let fullContent = ''
    let fullReasoning = ''

    try {
      const token = localStorage.getItem('token') || localStorage.getItem('gate_api_key') || ''
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      }
      if (token) {
        headers['Authorization'] = token.startsWith('Bearer ') || token.startsWith('sk-') ? token : `Bearer ${token}`
      }

      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers,
        signal: controller.signal,
        body: JSON.stringify({
          model: selectedModel,
          messages: newMessages
            .filter((m) => m.id !== 'welcome' && m.id !== assistantMsgId)
            .map((m) => ({ role: m.role, content: m.content })),
          stream: true,
        }),
      })

      if (!res.ok) {
        let errDesc = `请求失败 (${res.status})`
        try {
          const errData = await res.json()
          if (errData?.error?.message) errDesc = errData.error.message
        } catch {
          // ignore
        }
        throw new Error(errDesc)
      }

      if (!res.body) {
        throw new Error('未接收到可读流')
      }

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { value, done } = await reader.read()
        if (done) break

        if (firstTokenTime === 0) {
          firstTokenTime = Date.now() - startTime
        }

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed || trimmed.startsWith(': ping')) continue
          if (trimmed === 'data: [DONE]') continue

          if (trimmed.startsWith('data: ')) {
            try {
              const data = JSON.parse(trimmed.slice(6))
              const delta = data.choices?.[0]?.delta
              if (delta) {
                if (delta.reasoning_content) {
                  fullReasoning += delta.reasoning_content
                }
                if (delta.content) {
                  // 如果存在 <think> 标签，支持标准与非标准分离
                  fullContent += delta.content
                }

                // 提取 <think>...</think>
                let reasoningText = fullReasoning
                let mainContent = fullContent

                const thinkMatch = mainContent.match(/<think>([\s\S]*?)<\/think>/)
                if (thinkMatch) {
                  reasoningText = (reasoningText ? reasoningText + '\n' : '') + thinkMatch[1].trim()
                  mainContent = mainContent.replace(/<think>[\s\S]*?<\/think>/, '').trim()
                } else if (mainContent.includes('<think>')) {
                  const parts = mainContent.split('<think>')
                  reasoningText = parts[1]
                  mainContent = parts[0]
                }

                setMessages((prev) =>
                  prev.map((msg) =>
                    msg.id === assistantMsgId
                      ? {
                          ...msg,
                          content: mainContent,
                          reasoning_content: reasoningText,
                          isThinking: !mainContent && Boolean(reasoningText),
                          ttftMs: firstTokenTime,
                          durationMs: Date.now() - startTime,
                        }
                      : msg
                  )
                )
              }
            } catch {
              // 容忍非完整 JSON chunk
            }
          }
        }
      }
    } catch (err: unknown) {
      if ((err as Error).name !== 'AbortError') {
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === assistantMsgId
              ? {
                  ...msg,
                  content: `❌ 请求异常: ${(err as Error).message || '未知错误'}`,
                  isThinking: false,
                }
              : msg
          )
        )
      }
    } finally {
      setIsStreaming(false)
      abortControllerRef.current = null
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const currentModelMeta = models.find((m) => m.id === selectedModel)

  return (
    <div className="gate-chat-layout">
      {/* 顶部模型切换控制栏 */}
      <div className="gate-chat-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <Sparkles size={18} color="var(--color-primary)" />
          <select
            className="input-select"
            value={selectedModel}
            onChange={(e) => setSelectedModel(e.target.value)}
            disabled={isStreaming}
            style={{
              padding: '0.4rem 0.8rem',
              borderRadius: '6px',
              border: '1px solid var(--color-border-primary)',
              background: 'var(--color-bg-surface)',
              color: 'var(--color-text-primary)',
              fontWeight: 500,
              fontSize: '0.9rem',
            }}
          >
            {models.map((m) => (
              <option key={m.id} value={m.id}>
                {m.id} (倍率: {m.cost_multiplier}x)
              </option>
            ))}
          </select>
        </div>

        {currentModelMeta && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>
            <span className="badge" style={{ backgroundColor: 'var(--color-primary-subtle)', color: 'var(--color-primary)', border: '1px solid var(--color-primary-border)', padding: '2px 8px', borderRadius: '4px' }}>
              输入: {currentModelMeta.input_rate} / 缓存: {currentModelMeta.cache_hit_rate} / 输出: {currentModelMeta.output_rate}
            </span>
            <span>倍率: {currentModelMeta.cost_multiplier}x</span>
          </div>
        )}
      </div>

      {/* 聊天流容器 */}
      <div className="gate-chat-messages">
        {messages.map((msg) => {
          const isUser = msg.role === 'user'
          const hasReasoning = Boolean(msg.reasoning_content)
          const isExpanded = expandedThinking[msg.id] ?? false

          return (
            <div key={msg.id} className={`gate-message-item ${isUser ? 'gate-message-item--user' : ''}`}>
              <div className={`gate-avatar ${isUser ? 'gate-avatar--user' : 'gate-avatar--assistant'}`}>
                {isUser ? <User size={18} /> : <Bot size={18} />}
              </div>

              <div className="gate-message-body">
                {/* 思考过程折叠卡片 */}
                {hasReasoning && (
                  <div className="gate-thinking-card">
                    <div className="gate-thinking-header" onClick={() => toggleThinking(msg.id)}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                        <Zap size={14} color="var(--color-warning)" />
                        <span>{msg.isThinking ? '思考中...' : '已完成思考过程'}</span>
                      </div>
                      {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                    </div>
                    {isExpanded && (
                      <div className="gate-thinking-content">
                        {msg.reasoning_content}
                      </div>
                    )}
                  </div>
                )}

                {/* 消息正文气泡 */}
                <div className={`gate-bubble ${isUser ? 'gate-bubble--user' : 'gate-bubble--assistant'}`}>
                  {isUser ? (
                    msg.content
                  ) : (
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {msg.content || (msg.isThinking ? '*正在生成思考中...*' : '')}
                    </ReactMarkdown>
                  )}
                </div>

                {/* 耗时与 Token 统计 */}
                {!isUser && msg.durationMs && (
                  <div className="gate-message-meta">
                    {msg.ttftMs && <span>首字 TTFT: {msg.ttftMs}ms</span>}
                    <span>总耗时: {msg.durationMs}ms</span>
                  </div>
                )}
              </div>
            </div>
          )
        })}
        <div ref={messagesEndRef} />
      </div>

      {/* 底部输入框 */}
      <div className="gate-chat-input-area">
        <div className="gate-input-box">
          <textarea
            className="gate-textarea"
            placeholder="输入消息，Enter 发送，Shift + Enter 换行..."
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={isStreaming}
          />
          <div className="gate-input-footer">
            <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
              基于 100% 原始字节直通 · 严格协议感知路由分发
            </span>
            {isStreaming ? (
              <button
                type="button"
                className="btn btn-danger"
                onClick={handleStop}
                style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.4rem 0.85rem' }}
              >
                <Square size={14} />
                <span>停止生成</span>
              </button>
            ) : (
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleSend}
                disabled={!input.trim()}
                style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.4rem 0.85rem' }}
              >
                <Send size={14} />
                <span>发送</span>
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
