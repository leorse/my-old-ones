import {useEffect, useRef, useState} from 'react'
import {useTranslation} from 'react-i18next'
import {useQuery} from '@tanstack/react-query'
import {ipc} from '../ipc'
import {useDebounced} from '../queries'
import {useUI} from '../store/ui'
import {personLabel} from './helpers'

/** Recherche globale (Ctrl/Cmd+K) : nom, prénom, lieu, notes (spec §5.5). */
export function CommandPalette() {
    const {t} = useTranslation()
    const open = useUI((s) => s.paletteOpen)
    const setOpen = useUI((s) => s.setPaletteOpen)
    const select = useUI((s) => s.selectPerson)
    const [text, setText] = useState('')
    const [cursor, setCursor] = useState(0)
    const q = useDebounced(text, 120)
    const input = useRef<HTMLInputElement>(null)
    const results = useQuery({queryKey: ['search', q], queryFn: () => ipc.searchPersons(q), enabled: open && q.trim() !== ''})
    const list = results.data ?? []

    useEffect(() => {
        const h = (e: KeyboardEvent) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
                e.preventDefault()
                setOpen(!useUI.getState().paletteOpen)
            }
        }
        window.addEventListener('keydown', h)
        return () => window.removeEventListener('keydown', h)
    }, [setOpen])

    useEffect(() => {
        if (open) { setText(''); setCursor(0); input.current?.focus() }
    }, [open])

    if (!open) return null
    const pick = (id: string) => { select(id); setOpen(false) }
    return (
        <div className="dialog-backdrop top" role="presentation" onClick={() => setOpen(false)}>
            <div className="dialog palette" role="dialog" aria-modal="true" aria-label={t('toolbar.search')} onClick={(e) => e.stopPropagation()}>
                <input ref={input} aria-label={t('search.placeholder')} placeholder={t('search.placeholder')} value={text}
                       onChange={(e) => { setText(e.target.value); setCursor(0) }}
                       onKeyDown={(e) => {
                           if (e.key === 'Escape') setOpen(false)
                           else if (e.key === 'ArrowDown') { e.preventDefault(); setCursor((c) => Math.min(c + 1, list.length - 1)) }
                           else if (e.key === 'ArrowUp') { e.preventDefault(); setCursor((c) => Math.max(c - 1, 0)) }
                           else if (e.key === 'Enter' && list[cursor]) pick(list[cursor].id)
                       }}/>
                <ul role="listbox">
                    {list.map((p, i) => (
                        <li key={p.id} role="option" aria-selected={i === cursor}>
                            <button onMouseEnter={() => setCursor(i)} onClick={() => pick(p.id)}>{personLabel(p, t('common.unknown'))}</button>
                        </li>
                    ))}
                </ul>
                {q.trim() !== '' && !results.isFetching && list.length === 0 && <p className="muted">{t('search.none')}</p>}
            </div>
        </div>
    )
}
