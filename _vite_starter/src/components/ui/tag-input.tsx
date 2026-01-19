import { useState, useRef, type KeyboardEvent } from 'react'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from './badge'

interface TagInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  placeholder?: string
  disabled?: boolean
  className?: string
  validate?: (value: string) => boolean | string
  transform?: (value: string) => string
}

export function TagInput({
  value,
  onChange,
  placeholder = 'พิมพ์แล้วกด Enter...',
  disabled = false,
  className,
  validate,
  transform,
}: TagInputProps) {
  const [inputValue, setInputValue] = useState('')
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      addTag()
    } else if (e.key === 'Backspace' && inputValue === '' && value.length > 0) {
      // ลบ tag สุดท้ายถ้า input ว่าง
      removeTag(value.length - 1)
    }
  }

  const addTag = () => {
    const trimmed = inputValue.trim()
    if (!trimmed) return

    // Transform value if provided
    const finalValue = transform ? transform(trimmed) : trimmed

    // Check duplicate
    if (value.includes(finalValue)) {
      setError('มีโดเมนนี้อยู่แล้ว')
      return
    }

    // Validate if provided
    if (validate) {
      const result = validate(finalValue)
      if (result !== true) {
        setError(typeof result === 'string' ? result : 'รูปแบบโดเมนไม่ถูกต้อง')
        return
      }
    }

    onChange([...value, finalValue])
    setInputValue('')
    setError(null)
  }

  const removeTag = (index: number) => {
    onChange(value.filter((_, i) => i !== index))
  }

  const handleContainerClick = () => {
    inputRef.current?.focus()
  }

  return (
    <div className="space-y-1">
      <div
        className={cn(
          'flex flex-wrap gap-2 p-2 min-h-[42px] rounded-md border border-input bg-background cursor-text',
          'focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2',
          disabled && 'opacity-50 cursor-not-allowed',
          className
        )}
        onClick={handleContainerClick}
      >
        {value.map((tag, index) => (
          <Badge
            key={`${tag}-${index}`}
            variant="secondary"
            className="gap-1 pr-1"
          >
            <span className="font-mono text-xs">{tag}</span>
            {!disabled && (
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation()
                  removeTag(index)
                }}
                className="ml-1 rounded-full hover:bg-muted-foreground/20 p-0.5"
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </Badge>
        ))}
        <input
          ref={inputRef}
          type="text"
          value={inputValue}
          onChange={(e) => {
            setInputValue(e.target.value)
            setError(null)
          }}
          onKeyDown={handleKeyDown}
          onBlur={addTag}
          placeholder={value.length === 0 ? placeholder : ''}
          disabled={disabled}
          className={cn(
            'flex-1 min-w-[120px] bg-transparent outline-none text-sm',
            'placeholder:text-muted-foreground',
            disabled && 'cursor-not-allowed'
          )}
        />
      </div>
      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
    </div>
  )
}
