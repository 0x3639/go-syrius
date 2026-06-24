import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// `vi.mock` is hoisted above module-level code, so the spy referenced inside
// the factory must be created with `vi.hoisted` (the brief's plain `const`
// triggers a "Cannot access 'unlock' before initialization" hoisting error).
const { unlock } = vi.hoisted(() => ({ unlock: vi.fn().mockResolvedValue(undefined) }))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'Main' }]),
  Unlock: unlock,
}))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue'],
    template:
      '<input :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

import Unlock from './Unlock.vue'

beforeEach(() => setActivePinia(createPinia()))

describe('Unlock.vue', () => {
  it('unlocks with the entered password', async () => {
    const w = mount(Unlock)
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('button').trigger('click')
    await Promise.resolve()
    expect(unlock).toHaveBeenCalledWith('Main', 'pw')
  })
})
