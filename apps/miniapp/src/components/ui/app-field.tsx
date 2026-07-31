import { Input, Text, Textarea, View } from '@tarojs/components'
import './app-field.scss'

export interface AppFieldProps {
  label?: string
  value: string
  placeholder?: string
  multiline?: boolean
  inputType?: 'text' | 'number' | 'digit'
  maxLength?: number
  helperText?: string
  error?: string
  disabled?: boolean
  className?: string
  onInput: (value: string) => void
}

export function AppField({
  label,
  value,
  placeholder = '',
  multiline = false,
  inputType = 'text',
  maxLength = 140,
  helperText,
  error,
  disabled = false,
  className = '',
  onInput,
}: AppFieldProps) {
  return (
    <View className={`app-field ${error ? 'app-field--error' : ''} ${disabled ? 'app-field--disabled' : ''} ${className}`.trim()}>
      {label ? <Text className='app-field__label'>{label}</Text> : null}
      <View className={`app-field__control ${multiline ? 'app-field__control--multiline' : ''}`}>
        {multiline ? (
          <Textarea
            className='app-field__textarea'
            value={value}
            placeholder={placeholder}
            placeholderClass='app-field__placeholder'
            maxlength={maxLength}
            disabled={disabled}
            onInput={(event) => onInput(String(event.detail.value ?? ''))}
          />
        ) : (
          <Input
            className='app-field__input'
            type={inputType}
            value={value}
            placeholder={placeholder}
            placeholderClass='app-field__placeholder'
            maxlength={maxLength}
            disabled={disabled}
            onInput={(event) => onInput(String(event.detail.value ?? ''))}
          />
        )}
      </View>
      {error ? <Text className='app-field__message app-field__message--error'>{error}</Text> : null}
      {!error && helperText ? <Text className='app-field__message'>{helperText}</Text> : null}
    </View>
  )
}
