/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Loader2, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  applyModelReconcile,
  getModelAliases,
  previewModelReconcile,
  retireModelAliases,
} from '../../api'
import { modelsQueryKeys, vendorsQueryKeys } from '../../lib'
import type {
  ModelAlias,
  ModelReconcileConflict,
  ModelReconcileMatch,
  ModelReconcilePending,
} from '../../types'

type ModelReconcileDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function reconcileKey(item: ModelReconcileMatch) {
  return `${item.channel_id}\u0000${item.upstream_model}\u0000${item.canonical_model}`
}

function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
      {children}
    </div>
  )
}

function LoadingState() {
  return (
    <div className='flex min-h-48 items-center justify-center'>
      <Loader2 className='text-muted-foreground size-6 animate-spin' />
    </div>
  )
}

function RemoteDataState({
  loading,
  failed,
  children,
}: {
  loading: boolean
  failed: boolean
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  if (loading) return <LoadingState />
  if (failed) {
    return (
      <EmptyState>{t('Failed to load model reconciliation data')}</EmptyState>
    )
  }
  return children
}

function MatchRow({
  item,
  checked,
  onCheckedChange,
}: {
  item: ModelReconcileMatch
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  const { t } = useTranslation()
  return (
    <label className='hover:bg-muted/50 flex cursor-pointer items-start gap-3 rounded-lg border p-3'>
      <Checkbox
        checked={checked}
        onCheckedChange={(value) => onCheckedChange(Boolean(value))}
        aria-label={t('Select model match')}
      />
      <div className='min-w-0 flex-1 space-y-1'>
        <div className='flex flex-wrap items-center gap-2'>
          <span className='font-mono text-sm'>{item.upstream_model}</span>
          <span className='text-muted-foreground'>→</span>
          <span className='font-mono text-sm font-medium'>
            {item.canonical_model}
          </span>
          <StatusBadge
            label={
              item.match_type === 'exact'
                ? t('Exact match')
                : t('Normalized match')
            }
            variant={item.match_type === 'exact' ? 'success' : 'info'}
            size='sm'
            copyable={false}
          />
        </div>
        <p className='text-muted-foreground text-xs'>
          {item.channel_name} · {item.capabilities.join(', ')}
        </p>
      </div>
    </label>
  )
}

function PendingRow({ item }: { item: ModelReconcilePending }) {
  const { t } = useTranslation()
  return (
    <div className='rounded-lg border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-mono text-sm'>{item.upstream_model}</span>
        <StatusBadge
          label={t('Pending identification')}
          variant='warning'
          size='sm'
          copyable={false}
        />
      </div>
      <p className='text-muted-foreground mt-1 text-xs'>
        {item.channel_name} · {t('No unique catalog match was found')}
      </p>
    </div>
  )
}

function ConflictRow({ item }: { item: ModelReconcileConflict }) {
  const { t } = useTranslation()
  return (
    <div className='border-destructive/30 rounded-lg border p-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <AlertTriangle className='text-destructive size-4' />
        <span className='font-mono text-sm'>{item.upstream_model || '-'}</span>
        {item.canonical_model ? (
          <>
            <span className='text-muted-foreground'>→</span>
            <span className='font-mono text-sm'>{item.canonical_model}</span>
          </>
        ) : null}
      </div>
      <p className='text-muted-foreground mt-1 text-xs'>
        {item.channel_name} · {t(item.reason)}
        {item.detail ? ` · ${item.detail}` : ''}
      </p>
    </div>
  )
}

function AliasRow({
  item,
  retiring,
  onRetire,
}: {
  item: ModelAlias
  retiring: boolean
  onRetire: () => void
}) {
  const { t } = useTranslation()
  const expired =
    item.retire_after > 0 && item.retire_after <= Date.now() / 1000
  let statusLabel = t('Active')
  let statusVariant: 'neutral' | 'warning' | 'success' = 'success'
  if (item.status === 'retired') {
    statusLabel = t('Retired')
    statusVariant = 'neutral'
  } else if (expired) {
    statusLabel = t('Retirement due')
    statusVariant = 'warning'
  }
  return (
    <div className='flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center'>
      <div className='min-w-0 flex-1 space-y-1'>
        <div className='flex flex-wrap items-center gap-2'>
          <span className='font-mono text-sm'>{item.alias_name}</span>
          <span className='text-muted-foreground'>→</span>
          <span className='font-mono text-sm'>{item.canonical_model_name}</span>
          <StatusBadge
            label={statusLabel}
            variant={statusVariant}
            size='sm'
            copyable={false}
          />
        </div>
        <p className='text-muted-foreground text-xs'>
          {t('Requests')}: {item.request_count} · {t('Last used:')}{' '}
          {item.last_used_time
            ? new Date(item.last_used_time * 1000).toLocaleString()
            : '-'}{' '}
          · {t('Retirement date')}:{' '}
          {item.retire_after
            ? new Date(item.retire_after * 1000).toLocaleDateString()
            : '-'}
        </p>
      </div>
      {item.status === 'active' ? (
        <Button
          variant='outline'
          size='sm'
          disabled={retiring}
          onClick={onRetire}
        >
          {retiring ? <Loader2 className='size-4 animate-spin' /> : null}
          {t('Retire alias')}
        </Button>
      ) : null}
    </div>
  )
}

export function ModelReconcileDialog({
  open,
  onOpenChange,
}: ModelReconcileDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [excludedMatches, setExcludedMatches] = useState<Set<string>>(
    () => new Set()
  )
  const [isApplying, setIsApplying] = useState(false)
  const [retiringAlias, setRetiringAlias] = useState<string | null>(null)

  const previewQuery = useQuery({
    queryKey: ['models', 'reconcile-preview'],
    queryFn: () => previewModelReconcile(),
    enabled: open,
    retry: false,
  })
  const aliasesQuery = useQuery({
    queryKey: ['models', 'aliases'],
    queryFn: () => getModelAliases('all'),
    enabled: open,
    retry: false,
  })

  const preview = previewQuery.data?.data
  const safeMatches = preview?.safe_matches ?? []
  const pending = preview?.pending ?? []
  const conflicts = preview?.conflicts ?? []
  const aliases = aliasesQuery.data?.data ?? []
  const selectedMatches = safeMatches.filter(
    (item) => !excludedMatches.has(reconcileKey(item))
  )
  const allSelected =
    safeMatches.length > 0 && selectedMatches.length === safeMatches.length

  const refresh = async () => {
    setExcludedMatches(new Set())
    await Promise.all([previewQuery.refetch(), aliasesQuery.refetch()])
  }

  const applySelected = async () => {
    if (selectedMatches.length === 0) {
      toast.warning(t('Select at least one safe match'))
      return
    }
    setIsApplying(true)
    try {
      const response = await applyModelReconcile({
        items: selectedMatches.map((item) => ({
          channel_id: item.channel_id,
          upstream_model: item.upstream_model,
          canonical_model: item.canonical_model,
        })),
      })
      if (!response.success) {
        throw new Error(response.message || t('Failed to apply model matches'))
      }
      toast.success(t('Safe model matches applied'))
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() }),
        refresh(),
      ])
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to apply model matches')
      )
    } finally {
      setIsApplying(false)
    }
  }

  const retireAlias = async (aliasName: string) => {
    if (
      !window.confirm(
        t(
          'Retire this alias? Token model limits will be migrated to the canonical model first.'
        )
      )
    ) {
      return
    }
    setRetiringAlias(aliasName)
    try {
      const response = await retireModelAliases([aliasName])
      if (!response.success) {
        throw new Error(response.message || t('Failed to retire alias'))
      }
      toast.success(t('Alias retired'))
      await Promise.all([
        aliasesQuery.refetch(),
        previewQuery.refetch(),
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
      ])
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to retire alias')
      )
    } finally {
      setRetiringAlias(null)
    }
  }

  const isLoading = previewQuery.isLoading && aliasesQuery.isLoading
  const previewFailed = Boolean(
    previewQuery.error || previewQuery.data?.success === false
  )
  const aliasesFailed = Boolean(
    aliasesQuery.error || aliasesQuery.data?.success === false
  )
  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) setExcludedMatches(new Set())
    onOpenChange(nextOpen)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t('Model identification')}
      description={t(
        'Preview catalog matches before changing channel model names or creating compatibility aliases.'
      )}
      contentHeight='min(68vh, 680px)'
      contentClassName='sm:max-w-4xl'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => void refresh()}>
            <RefreshCw className='size-4' />
            {t('Refresh preview')}
          </Button>
          <Button
            onClick={() => void applySelected()}
            disabled={isApplying || selectedMatches.length === 0}
          >
            {isApplying ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Apply selected safe matches')} ({selectedMatches.length})
          </Button>
        </>
      }
    >
      {isLoading && <LoadingState />}
      {!isLoading && (
        <Tabs defaultValue='safe' className='space-y-3'>
          <TabsList className='grid h-auto w-full grid-cols-2 sm:grid-cols-4'>
            <TabsTrigger value='safe'>
              {t('Safe matches')} ({safeMatches.length})
            </TabsTrigger>
            <TabsTrigger value='pending'>
              {t('Pending identification')} ({pending.length})
            </TabsTrigger>
            <TabsTrigger value='conflicts'>
              {t('Match conflicts')} ({conflicts.length})
            </TabsTrigger>
            <TabsTrigger value='aliases'>
              {t('Legacy aliases')} ({aliases.length})
            </TabsTrigger>
          </TabsList>

          <TabsContent value='safe' className='space-y-3'>
            <RemoteDataState
              loading={previewQuery.isLoading}
              failed={previewFailed}
            >
              {safeMatches.length > 0 ? (
                <>
                  <label className='flex items-center gap-2 text-sm font-medium'>
                    <Checkbox
                      checked={allSelected}
                      indeterminate={selectedMatches.length > 0 && !allSelected}
                      onCheckedChange={(value) => {
                        setExcludedMatches(
                          value
                            ? new Set()
                            : new Set(safeMatches.map(reconcileKey))
                        )
                      }}
                      aria-label={t('Select all safe matches')}
                    />
                    {t('Select all safe matches')}
                  </label>
                  <div className='space-y-2'>
                    {safeMatches.map((item) => {
                      const key = reconcileKey(item)
                      return (
                        <MatchRow
                          key={key}
                          item={item}
                          checked={!excludedMatches.has(key)}
                          onCheckedChange={(checked) => {
                            setExcludedMatches((current) => {
                              const next = new Set(current)
                              if (checked) next.delete(key)
                              else next.add(key)
                              return next
                            })
                          }}
                        />
                      )
                    })}
                  </div>
                </>
              ) : (
                <EmptyState>
                  {t('No safe matches need to be applied')}
                </EmptyState>
              )}
            </RemoteDataState>
          </TabsContent>

          <TabsContent value='pending' className='space-y-2'>
            <RemoteDataState
              loading={previewQuery.isLoading}
              failed={previewFailed}
            >
              {pending.length > 0 ? (
                pending.map((item) => (
                  <PendingRow
                    key={`${item.channel_id}-${item.upstream_model}`}
                    item={item}
                  />
                ))
              ) : (
                <EmptyState>
                  {t('No models are pending identification')}
                </EmptyState>
              )}
            </RemoteDataState>
          </TabsContent>

          <TabsContent value='conflicts' className='space-y-2'>
            <RemoteDataState
              loading={previewQuery.isLoading}
              failed={previewFailed}
            >
              {conflicts.length > 0 ? (
                conflicts.map((item) => (
                  <ConflictRow
                    key={`${item.channel_id}-${item.upstream_model}-${item.canonical_model}-${item.reason}`}
                    item={item}
                  />
                ))
              ) : (
                <EmptyState>{t('No model matching conflicts')}</EmptyState>
              )}
            </RemoteDataState>
          </TabsContent>

          <TabsContent value='aliases' className='space-y-2'>
            <RemoteDataState
              loading={aliasesQuery.isLoading}
              failed={aliasesFailed}
            >
              {aliases.length > 0 ? (
                aliases.map((item) => (
                  <AliasRow
                    key={item.id}
                    item={item}
                    retiring={retiringAlias === item.alias_name}
                    onRetire={() => void retireAlias(item.alias_name)}
                  />
                ))
              ) : (
                <EmptyState>{t('No legacy aliases')}</EmptyState>
              )}
            </RemoteDataState>
          </TabsContent>
        </Tabs>
      )}
    </Dialog>
  )
}
