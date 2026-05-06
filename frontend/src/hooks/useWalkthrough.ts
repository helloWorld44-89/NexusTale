import { useState, useEffect } from 'react'

const STORAGE_KEY = 'nexustale_tour_done'
export const TOUR_STEPS = 6

export function useWalkthrough(ready: boolean) {
  const [step, setStep] = useState<number | null>(null)

  useEffect(() => {
    if (!ready) return
    if (!localStorage.getItem(STORAGE_KEY)) {
      setStep(0)
    }
  }, [ready])

  const next = () => {
    setStep((s) => {
      if (s === null) return null
      if (s >= TOUR_STEPS - 1) {
        localStorage.setItem(STORAGE_KEY, 'true')
        return null
      }
      return s + 1
    })
  }

  const skip = () => {
    localStorage.setItem(STORAGE_KEY, 'true')
    setStep(null)
  }

  const restart = () => {
    localStorage.removeItem(STORAGE_KEY)
    setStep(0)
  }

  return {
    active: step !== null,
    step: step ?? 0,
    next,
    skip,
    restart,
  }
}
