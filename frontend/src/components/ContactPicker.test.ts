import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const ADDR = 'z1' + 'q'.repeat(38)
const ListContacts = vi.hoisted(() => vi.fn().mockResolvedValue([{ name: 'Alice', address: 'z1' + 'q'.repeat(38) }]))
const AddContact = vi.hoisted(() => vi.fn().mockResolvedValue([]))
const DeleteContact = vi.hoisted(() => vi.fn().mockResolvedValue([]))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ ListContacts, AddContact, DeleteContact }))
import ContactPicker from './ContactPicker.vue'

const flush = () => new Promise((r) => setTimeout(r))

beforeEach(() => {
  setActivePinia(createPinia())
  ListContacts.mockClear()
  AddContact.mockClear()
  DeleteContact.mockClear()
})

describe('ContactPicker', () => {
  it('lists saved contacts and emits select on click', async () => {
    const w = mount(ContactPicker, { props: { open: true } })
    await flush() // open watcher → load
    expect(w.text()).toContain('Alice')
    await w.find('[aria-label="select Alice"]').trigger('click')
    expect(w.emitted('select')![0]).toEqual([ADDR])
  })

  it('prefills the add form with a valid current address and saves a contact', async () => {
    const w = mount(ContactPicker, { props: { open: true, currentAddress: ADDR } })
    await flush()
    expect((w.find('input[aria-label="contact address"]').element as HTMLInputElement).value).toBe(ADDR)
    await w.find('input[aria-label="contact name"]').setValue('Bob')
    await w.find('button[aria-label="save contact"]').trigger('click')
    expect(AddContact).toHaveBeenCalledWith('Bob', ADDR)
  })

  it('deletes a contact', async () => {
    const w = mount(ContactPicker, { props: { open: true } })
    await flush()
    await w.find('[aria-label="delete Alice"]').trigger('click')
    expect(DeleteContact).toHaveBeenCalledWith(ADDR)
  })

  it('renders nothing when closed', () => {
    const w = mount(ContactPicker, { props: { open: false } })
    expect(w.text()).not.toContain('Add address')
  })
})
