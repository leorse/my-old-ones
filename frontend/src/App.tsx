import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import './i18n'
import {Shell} from './components/Shell'
import {ImportDialog} from './features/gedcom/ImportDialog'

const queryClient = new QueryClient({
    defaultOptions: {queries: {staleTime: 5_000, retry: false, refetchOnWindowFocus: false}},
})

export default function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <Shell/>
            <ImportDialog/>
        </QueryClientProvider>
    )
}
