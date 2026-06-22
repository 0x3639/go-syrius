import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

const mocks = vi.hoisted(() => ({
  GetSentinel: vi.fn(),
  GetDepositedQsr: vi.fn(),
  GetSentinelReward: vi.fn(),
  PrepareDepositQsr: vi.fn(), PrepareRegisterSentinel: vi.fn(),
  PrepareCollectSentinelReward: vi.fn(), PrepareRevokeSentinel: vi.fn(), PrepareWithdrawQsr: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => mocks)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Sentinels from './Sentinels.svelte'

describe('Sentinels', () => {
  it('shows Deposit (not Register) when escrowed QSR is below 50,000', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: '', registrationTimestamp: 0, isRevocable: false, revokeCooldown: 0, active: false })
    mocks.GetDepositedQsr.mockResolvedValue('1000000000000') // 10,000 QSR < 50,000
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    expect(await screen.findByRole('button', { name: /deposit qsr/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /register sentinel/i })).toBeNull()
  })

  it('shows Register when escrowed QSR reaches 50,000', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: '', registrationTimestamp: 0, isRevocable: false, revokeCooldown: 0, active: false })
    mocks.GetDepositedQsr.mockResolvedValue('5000000000000') // exactly 50,000 QSR
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    expect(await screen.findByRole('button', { name: /register sentinel/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /deposit qsr/i })).toBeNull()
  })

  it('shows Withdraw in the ready-to-register state (deposited >= 50,000, no sentinel)', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: '', registrationTimestamp: 0, isRevocable: false, revokeCooldown: 0, active: false })
    mocks.GetDepositedQsr.mockResolvedValue('5000000000000') // exactly 50,000 QSR
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    expect(await screen.findByRole('button', { name: /register sentinel/i })).toBeTruthy()
    // user may deposit the full collateral but choose not to register — they must
    // still be able to recover it.
    expect(screen.getByRole('button', { name: /withdraw qsr/i })).toBeTruthy()
  })

  it('hides Withdraw when nothing is deposited and no sentinel', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: '', registrationTimestamp: 0, isRevocable: false, revokeCooldown: 0, active: false })
    mocks.GetDepositedQsr.mockResolvedValue('0')
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    expect(await screen.findByRole('button', { name: /deposit qsr/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /withdraw qsr/i })).toBeNull()
  })

  it('shows status + disabled Revoke (not yet revocable) for an active sentinel', async () => {
    mocks.GetSentinel.mockResolvedValue({ owner: 'z1qtest', registrationTimestamp: 1718000000, isRevocable: false, revokeCooldown: 100, active: true })
    mocks.GetDepositedQsr.mockResolvedValue('0')
    mocks.GetSentinelReward.mockResolvedValue({ znn: '0', qsr: '0' })
    render(Sentinels)
    const revoke = await screen.findByRole('button', { name: /revoke sentinel/i }) as HTMLButtonElement
    expect(revoke.disabled).toBe(true)
    expect(screen.queryByRole('button', { name: /register sentinel/i })).toBeNull()
  })
})
