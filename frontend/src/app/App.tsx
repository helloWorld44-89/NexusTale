import { useEffect } from 'react'
import { AppRouter } from './router'
import { useThemeStore } from './store/themeStore'

export default function App() {
  const init = useThemeStore((s) => s.init)

  useEffect(() => {
    init()
  }, [])

  return <AppRouter />
}
