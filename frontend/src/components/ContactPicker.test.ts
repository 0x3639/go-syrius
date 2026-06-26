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
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ ListContacts }))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
import ContactPicker from './ContactPicker.vue'

const flush = () => new Promise((r) => setTimeout(r))

beforeEach(() => {
  setActivePinia(createPinia())
  ListContacts.mockClear()
  push.mockClear()
})

describe('ContactPicker', () => {
  it('lists contacts and emits select on click', async () => {
    const w = mount(ContactPicker, { props: { open: true } })
    await flush()
    expect(w.text()).toContain('Alice')
    await w.find('[aria-label="select Alice"]').trigger('click')
    expect(w.emitted('select')![0]).toEqual([ADDR])
  })

  it('filters the list by search', async () => {
    const w = mount(ContactPicker, { props: { open: true } })
    await flush()
    await w.find('input[aria-label="search addresses"]').setValue('bob')
    expect(w.text()).toContain('Bob')
    expect(w.text()).not.toContain('Alice')
  })

  it('navigates to the address book to manage', async () => {
    const w = mount(ContactPicker, { props: { open: true } })
    await flush()
    await w.find('button[aria-label="manage address book"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/address-book')
  })

  it('renders nothing when closed', () => {
    const w = mount(ContactPicker, { props: { open: false } })
    expect(w.text()).not.toContain('Address book')
  })
})
