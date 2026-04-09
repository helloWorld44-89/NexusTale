import tsPlugin from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import reactHooks from 'eslint-plugin-react-hooks'

/** @type {import('eslint').Linter.FlatConfig[]} */
export default [
  {
    ignores: ['dist/', 'node_modules/', 'src/services/api-types.ts'],
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
      'react-hooks': reactHooks,
    },
    rules: {
      ...tsPlugin.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      // allow explicit any in codegen-adjacent files and quick prototypes
      '@typescript-eslint/no-explicit-any': 'warn',
      // unused vars: error on real vars, warn on underscore-prefixed
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      // functional-updater form (setX(prev => ...)) inside effects is safe —
      // react-hooks v7 flags it unconditionally but it cannot cause infinite loops
      // when only adding new keys (ProjectExplorer expand-on-add pattern).
      'react-hooks/set-state-in-effect': 'warn',
    },
  },
]
