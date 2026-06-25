import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const GetPillarList = vi.hoisted(() => vi.fn())
const GetDelegation = vi.hoisted(() => vi.fn())
const GetPillarReward = vi.hoisted(() => vi.fn())
const GetPlasmaInfo = vi.hoisted(() => vi.fn())
const GetFusionEntries = vi.hoisted(() => vi.fn())
const EstimatePlasma = vi.hoisted(() => vi.fn())
const GetStakeList = vi.hoisted(() => vi.fn())
const GetUncollectedReward = vi.hoisted(() => vi.fn())
const GetSentinel = vi.hoisted(() => vi.fn())
const GetDepositedQsr = vi.hoisted(() => vi.fn())
const GetSentinelReward = vi.hoisted(() => vi.fn())
const GetProjects = vi.hoisted(() => vi.fn())
const GetProject = vi.hoisted(() => vi.fn())
const GetVotablePillars = vi.hoisted(() => vi.fn())

vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetPillarList, GetDelegation, GetPillarReward,
  GetPlasmaInfo, GetFusionEntries, EstimatePlasma,
  GetStakeList, GetUncollectedReward,
  GetSentinel, GetDepositedQsr, GetSentinelReward,
  GetProjects, GetProject, GetVotablePillars,
}))

import { usePillarStore } from './pillar'
import { usePlasmaStore } from './plasma'
import { useStakeStore } from './stake'
import { useSentinelStore } from './sentinel'
import { useAcceleratorStore } from './accelerator'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('pillar store', () => {
  it('refresh loads pillars, delegation, reward', async () => {
    GetPillarList.mockResolvedValue([{ name: 'P1' }])
    GetDelegation.mockResolvedValue({ name: 'P1' })
    GetPillarReward.mockResolvedValue({ znn: '1', qsr: '2' })
    const s = usePillarStore()
    await s.refresh()
    expect(s.pillars).toEqual([{ name: 'P1' }])
    expect(s.delegation).toEqual({ name: 'P1' })
    expect(s.reward).toEqual({ znn: '1', qsr: '2' })
  })
  it('refresh swallows errors, keeps state', async () => {
    GetPillarList.mockRejectedValue(new Error('locked'))
    const s = usePillarStore()
    await s.refresh()
    expect(s.pillars).toEqual([])
  })
})

describe('plasma store', () => {
  it('refresh loads info and fusion entries', async () => {
    GetPlasmaInfo.mockResolvedValue({ currentPlasma: '100' })
    GetFusionEntries.mockResolvedValue([{ id: 'f1' }])
    const s = usePlasmaStore()
    await s.refresh()
    expect(s.info).toEqual({ currentPlasma: '100' })
    expect(s.fusionEntries).toEqual([{ id: 'f1' }])
  })
  it('estimate returns mocked number', async () => {
    EstimatePlasma.mockResolvedValue(2100)
    const s = usePlasmaStore()
    expect(await s.estimate('50')).toBe(2100)
  })
  it('estimate returns 0 on error', async () => {
    EstimatePlasma.mockRejectedValue(new Error('nope'))
    const s = usePlasmaStore()
    expect(await s.estimate('50')).toBe(0)
  })
})

describe('stake store', () => {
  it('refresh sets stakeInfo and reward', async () => {
    GetStakeList.mockResolvedValue({ totalAmount: '10' })
    GetUncollectedReward.mockResolvedValue({ znn: '1', qsr: '2' })
    const s = useStakeStore()
    await s.refresh()
    expect(s.stakeInfo).toEqual({ totalAmount: '10' })
    expect(s.reward).toEqual({ znn: '1', qsr: '2' })
  })
})

describe('sentinel store', () => {
  it('refresh sets sentinel, depositedQsr, reward', async () => {
    GetSentinel.mockResolvedValue({ active: true })
    GetDepositedQsr.mockResolvedValue('5000')
    GetSentinelReward.mockResolvedValue({ znn: '3', qsr: '4' })
    const s = useSentinelStore()
    await s.refresh()
    expect(s.sentinel).toEqual({ active: true })
    expect(s.depositedQsr).toBe('5000')
    expect(s.reward).toEqual({ znn: '3', qsr: '4' })
  })
})

describe('accelerator store', () => {
  it('loadProjects sets projects from GetProjects().list', async () => {
    GetProjects.mockResolvedValue({ list: [{ id: 'p1' }, { id: 'p2' }] })
    const s = useAcceleratorStore()
    await s.loadProjects()
    expect(GetProjects).toHaveBeenCalledWith(0, 20)
    expect(s.projects).toEqual([{ id: 'p1' }, { id: 'p2' }])
    expect(s.error).toBe('')
  })
  it('loadProjects surfaces error', async () => {
    GetProjects.mockRejectedValue(new Error('boom'))
    const s = useAcceleratorStore()
    await s.loadProjects()
    expect(s.error).toBe('boom')
  })
  it('openProject sets selectedProject', async () => {
    GetProject.mockResolvedValue({ id: 'p1' })
    const s = useAcceleratorStore()
    await s.openProject('p1')
    expect(s.selectedProject).toEqual({ id: 'p1' })
  })
  it('loadVotablePillars sets [] on error', async () => {
    GetVotablePillars.mockRejectedValue(new Error('locked'))
    const s = useAcceleratorStore()
    await s.loadVotablePillars()
    expect(s.votablePillars).toEqual([])
  })
})
