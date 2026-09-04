import { zodResolver } from '@hookform/resolvers/zod'
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
import {
  AiMagicIcon,
  Copy01Icon,
  Loading03Icon,
  ViewIcon,
  ViewOffIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import {
  generateInternalProvisionToken,
  getInternalProvisionTokenStatus,
  revealInternalProvisionToken,
  updateInternalProvisionToken,
} from '../api'
import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

function createInternalProvisionTokenSchema(t: (key: string) => string) {
  return z.object({
    token: z
      .string()
      .trim()
      .min(32, t('Token must contain at least 32 characters'))
      .max(512, t('Token must not exceed 512 characters'))
      .regex(/^\S+$/, t('Token cannot contain whitespace')),
  })
}

type InternalProvisionTokenFormValues = z.infer<
  ReturnType<typeof createInternalProvisionTokenSchema>
>

const statusQueryKey = ['internal-provision-token-status'] as const

export function InternalServiceAuthSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [tokenVisible, setTokenVisible] = useState(false)
  const form = useForm<InternalProvisionTokenFormValues>({
    resolver: zodResolver(createInternalProvisionTokenSchema(t)),
    defaultValues: { token: '' },
    mode: 'onChange',
  })
  const statusQuery = useQuery({
    queryKey: statusQueryKey,
    queryFn: getInternalProvisionTokenStatus,
  })
  const status = statusQuery.data?.data

  const saveMutation = useMutation({
    mutationFn: (token: string) => updateInternalProvisionToken(token),
    onSuccess: async (response) => {
      if (!response.success) {
        throw new Error(response.message || t('Failed to update setting'))
      }
      form.reset({ token: '' })
      setTokenVisible(false)
      await queryClient.invalidateQueries({ queryKey: statusQueryKey })
      toast.success(t('Internal provisioning token updated'))
    },
    onError: (error: Error) => toast.error(error.message),
  })
  const generateMutation = useMutation({
    mutationFn: generateInternalProvisionToken,
    onSuccess: (response) => {
      const generatedToken = response.data?.token
      if (!response.success || !generatedToken) {
        toast.error(response.message || t('Failed to generate token'))
        return
      }
      form.setValue('token', generatedToken, {
        shouldDirty: true,
        shouldValidate: true,
      })
      setTokenVisible(true)
      toast.info(t('Generated token is not saved yet'))
    },
    onError: (error: Error) => toast.error(error.message),
  })

  const fetchToken = useCallback(
    async (proofToken?: string) => {
      const response = await revealInternalProvisionToken(proofToken)
      const revealedToken = response.data?.token
      if (!response.success || !revealedToken) {
        throw new Error(response.message || t('Failed to reveal token'))
      }
      form.setValue('token', revealedToken, {
        shouldDirty: false,
        shouldValidate: true,
      })
      setTokenVisible(true)
      return response
    },
    [form, t]
  )
  const {
    open: verificationOpen,
    methods: verificationMethods,
    state: verificationState,
    withVerification,
    executeVerification,
    cancel: cancelVerification,
    setCode: setVerificationCode,
    switchMethod: switchVerificationMethod,
  } = useSecureVerification()

  const revealToken = async () => {
    try {
      await withVerification(fetchToken, {
        scope: 'internal-provision.token.read',
        preferredMethod: 'passkey',
        title: t('Verify to view internal provisioning token'),
        description: t(
          'Use Passkey or 2FA to confirm your identity before revealing this server credential.'
        ),
      })
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to reveal token')
      )
    }
  }

  const copyToken = async () => {
    const copied = await copyToClipboard(form.getValues('token'))
    if (copied) toast.success(t('Copied'))
  }

  const onSubmit = (values: InternalProvisionTokenFormValues) => {
    saveMutation.mutate(values.token.trim())
  }

  const configuredSource =
    status?.source === 'database'
      ? t('Encrypted database setting')
      : t('Environment variable')

  return (
    <SettingsSection title={t('Internal Service Authentication')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={saveMutation.isPending}
            isSaveDisabled={
              !form.formState.isDirty || !status?.encryption_ready
            }
            saveLabel='Save internal token'
          />

          {!status?.encryption_ready && !statusQuery.isLoading ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Stable encryption secret required')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Set a stable SESSION_SECRET or CRYPTO_SECRET and restart the service before saving this credential.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}

          {status?.encryption_ready ? (
            <Alert>
              <AlertTitle>{t('Credential rotation')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Saving activates the new token immediately. Update every calling service to use the same value.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}

          <FormField
            control={form.control}
            name='token'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <div className='flex flex-wrap items-center gap-2'>
                  <FormLabel>NEW_API_INTERNAL_TOKEN</FormLabel>
                  <Badge variant={status?.configured ? 'secondary' : 'outline'}>
                    {status?.configured ? t('Configured') : t('Not configured')}
                  </Badge>
                  {status?.configured ? (
                    <span className='text-muted-foreground text-xs'>
                      {configuredSource}
                    </span>
                  ) : null}
                </div>
                <FormControl>
                  <InputGroup>
                    <InputGroupInput
                      {...field}
                      type={tokenVisible ? 'text' : 'password'}
                      aria-label='NEW_API_INTERNAL_TOKEN'
                      placeholder={
                        status?.configured
                          ? t('Configured — reveal to view')
                          : t('Generate or enter a server-only token')
                      }
                      autoComplete='new-password'
                      className='font-mono'
                    />
                    <InputGroupAddon align='inline-end'>
                      <InputGroupButton
                        aria-label={
                          tokenVisible ? t('Hide token') : t('Reveal token')
                        }
                        onClick={() => {
                          if (field.value) {
                            setTokenVisible((visible) => !visible)
                            return
                          }
                          void revealToken()
                        }}
                        disabled={!status?.configured && !field.value}
                      >
                        <HugeiconsIcon
                          icon={tokenVisible ? ViewOffIcon : ViewIcon}
                          strokeWidth={2}
                        />
                      </InputGroupButton>
                      <InputGroupButton
                        aria-label={t('Copy token')}
                        onClick={() => void copyToken()}
                        disabled={!field.value}
                      >
                        <HugeiconsIcon icon={Copy01Icon} strokeWidth={2} />
                      </InputGroupButton>
                    </InputGroupAddon>
                  </InputGroup>
                </FormControl>
                <FormDescription>
                  {t(
                    'Authenticates trusted server-to-server calls to POST /api/internal/provision. The saved value is encrypted at rest.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='flex flex-wrap gap-2' data-settings-form-span='full'>
            <Button
              type='button'
              variant='outline'
              onClick={() => generateMutation.mutate()}
              disabled={generateMutation.isPending || !status?.encryption_ready}
            >
              <HugeiconsIcon
                icon={generateMutation.isPending ? Loading03Icon : AiMagicIcon}
                strokeWidth={2}
                data-icon='inline-start'
                className={
                  generateMutation.isPending ? 'animate-spin' : undefined
                }
              />
              {t('Generate secure token')}
            </Button>
            {status?.configured ? (
              <Button
                type='button'
                variant='outline'
                onClick={() => void revealToken()}
              >
                <HugeiconsIcon
                  icon={ViewIcon}
                  strokeWidth={2}
                  data-icon='inline-start'
                />
                {t('Reveal saved token')}
              </Button>
            ) : null}
          </div>
        </SettingsForm>
      </Form>

      <SecureVerificationDialog
        open={verificationOpen}
        onOpenChange={(open) => {
          if (!open) cancelVerification()
        }}
        methods={verificationMethods}
        state={verificationState}
        onVerify={async (method, code) => {
          await executeVerification(method, code)
        }}
        onCancel={cancelVerification}
        onCodeChange={setVerificationCode}
        onMethodChange={switchVerificationMethod}
      />
    </SettingsSection>
  )
}
