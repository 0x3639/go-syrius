import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const ADDR = 'z1' + 'q'.repeat(38)
const ListContacts = vi.hoisted(() =>
  vi.fn().mockResolvedValue([
    { name: 'Alice', address: 'z1' + 'q'.repeat(38) },
    { name: 'Bob', address: 'z1' + 'r'.repeat(38) },
  ]),
)
const AddContact = vi.hoisted(() => vi.fn().mockResolvedValue([]))
const DeleteContact = vi.hoisted(() => vi.fn().mockResolvedValue([]))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ ListContacts, AddContact, DeleteContact }))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot /></div>' },
  CardContent: { template: '<div><slot /></div>' },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>' },
  Input: {
    props: ['modelValue'],
    template: '<input :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import AddressBook from './AddressBook.vue'

const flush = () => new Promise((r) => setTimeout(r))

beforeEach(() => {
  setActivePinia(createPinia())
  ListContacts.mockClear()
  AddContact.mockClear()
  DeleteContact.mockClear()
})

describe('AddressBook', () => {
  it('lists saved contacts', async () => {
    const w = mount(AddressBook)
    await flush()
    expect(w.text()).toContain('Alice')
    expect(w.text()).toContain('Bob')
  })

  it('filters by search', async () => {
    const w = mount(AddressBook)
    await flush()
    await w.find('input[aria-label="search addresses"]').setValue('bob')
    expect(w.text()).toContain('Bob')
    expect(w.text()).not.toContain('Alice')
  })

  it('adds a contact', async () => {
    const w = mount(AddressBook)
    await flush()
    await w.find('input[aria-label="contact name"]').setValue('Carol')
    await w.find('input[aria-label="contact address"]').setValue(ADDR)
    await w.find('button[aria-label="save contact"]').trigger('click')
    expect(AddContact).toHaveBeenCalledWith('Carol', ADDR)
  })

  it('deletes a contact', async () => {
    const w = mount(AddressBook)
    await flush()
    await w.find('button[aria-label="delete Alice"]').trigger('click')
    expect(DeleteContact).toHaveBeenCalledWith(ADDR)
  })
})
