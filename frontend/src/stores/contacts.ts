import { defineStore } from 'pinia'
import * as Cfg from '../../wailsjs/go/app/ConfigService'

export type Contact = { name: string; address: string }

// Address book — saved name → address entries, persisted in settings (backend
// validates the address authoritatively on add).
export const useContactsStore = defineStore('contacts', {
  state: () => ({ items: [] as Contact[] }),
  actions: {
    async load() {
      try {
        this.items = (await Cfg.ListContacts()) as unknown as Contact[]
      } catch {
        this.items = []
      }
    },
    async add(name: string, address: string) {
      this.items = (await Cfg.AddContact(name, address)) as unknown as Contact[]
    },
    async remove(address: string) {
      this.items = (await Cfg.DeleteContact(address)) as unknown as Contact[]
    },
  },
})
