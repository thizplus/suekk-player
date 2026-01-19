import { useQuery } from '@tanstack/react-query'
import { workerService } from './service'

// Query Key Factory
export const workerKeys = {
  all: ['workers'] as const,
  online: () => [...workerKeys.all, 'online'] as const,
}

// ==================== Online Workers (NATS KV - Auto-Discovery) ====================

export function useOnlineWorkers() {
  return useQuery({
    queryKey: workerKeys.online(),
    queryFn: () => workerService.getOnlineWorkers(),
    refetchInterval: 5000, // Refresh every 5 seconds
  })
}
