import js from '@eslint/js'
import vue from 'eslint-plugin-vue'
import typescript from '@typescript-eslint/eslint-plugin'
import typescriptParser from '@typescript-eslint/parser'
import vueParser from 'vue-eslint-parser'
import prettier from 'eslint-plugin-prettier'
import prettierConfig from 'eslint-config-prettier'

export default [
  // 基础 JavaScript 推荐配置
  js.configs.recommended,

  // Vue 3 基础配置
  ...vue.configs['flat/essential'],

  // Prettier 配置
  prettierConfig,

  {
    files: ['**/*.{js,mjs,cjs,ts,vue}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      parser: vueParser,
      parserOptions: {
        parser: typescriptParser,
        extraFileExtensions: ['.vue'],
      },
      globals: {
        h: 'readonly',
        unref: 'readonly',
        provide: 'readonly',
        inject: 'readonly',
        markRaw: 'readonly',
        defineAsyncComponent: 'readonly',
        nextTick: 'readonly',
        useRoute: 'readonly',
        useRouter: 'readonly',
        Message: 'readonly',
        $loadingBar: 'readonly',
        $message: 'readonly',
        $dialog: 'readonly',
        $notification: 'readonly',
        $modal: 'readonly',
        Models: 'readonly',
        NodeJS: 'readonly',
        Enums: 'readonly',
      },
    },
    plugins: {
      '@typescript-eslint': typescript,
      prettier,
    },
    rules: {
      'n/prefer-global/process': 'off',
      'no-undef': 'error',
      'no-fallthrough': 'off',
      'vue/block-order': 'off',
      '@typescript-eslint/no-this-alias': 'off',
      'prefer-promise-reject-errors': 'off',
      'vue/multi-word-component-names': 'off',
    },
  },

  // TypeScript 文件特定配置
  {
    files: ['**/*.{ts,tsx,vue}'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: typescriptParser,
        project: './tsconfig.json',
        extraFileExtensions: ['.vue'],
      },
    },
    rules: {
      ...typescript.configs.recommended.rules,
      // 启用类型感知的规则
      '@typescript-eslint/no-unsafe-call': 'error',
    },
  },

  // 忽略文件
  {
    ignores: ['dist/**', 'node_modules/**', '*.config.js', '*.config.ts', '.stylelintrc.cjs'],
  },
]
