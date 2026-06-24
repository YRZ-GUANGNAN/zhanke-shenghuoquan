import { create } from 'zustand'
import type { BuyerDish, BuyerMerchant } from '../services/g-takeout-buyer'

export interface CartItem {
  dishPublicId: string
  name: string
  priceCents: number
  quantity: number
  imageContentUrl?: string
}

interface TakeoutCartState {
  merchantPublicId: string
  merchantName: string
  items: CartItem[]
  addDish: (merchant: BuyerMerchant, dish: BuyerDish) => void
  increment: (dishPublicId: string) => void
  decrement: (dishPublicId: string) => void
  remove: (dishPublicId: string) => void
  clear: () => void
  totalQuantity: () => number
  totalCents: () => number
}

export const useTakeoutCartStore = create<TakeoutCartState>((set, get) => ({
  merchantPublicId: '',
  merchantName: '',
  items: [],
  addDish: (merchant, dish) => {
    set((state) => {
      const sameMerchant = state.merchantPublicId === '' || state.merchantPublicId === merchant.merchantPublicId
      const baseItems = sameMerchant ? state.items : []
      const existing = baseItems.find((item) => item.dishPublicId === dish.dishPublicId)
      const nextItems = existing
        ? baseItems.map((item) => item.dishPublicId === dish.dishPublicId ? { ...item, quantity: item.quantity + 1 } : item)
        : [
            ...baseItems,
            {
              dishPublicId: dish.dishPublicId,
              name: dish.name,
              priceCents: dish.priceCents,
              quantity: 1,
              imageContentUrl: dish.imageContentUrl
            }
          ]
      return {
        merchantPublicId: merchant.merchantPublicId,
        merchantName: merchant.name,
        items: nextItems
      }
    })
  },
  increment: (dishPublicId) => set((state) => ({
    items: state.items.map((item) => item.dishPublicId === dishPublicId ? { ...item, quantity: item.quantity + 1 } : item)
  })),
  decrement: (dishPublicId) => set((state) => ({
    items: state.items
      .map((item) => item.dishPublicId === dishPublicId ? { ...item, quantity: item.quantity - 1 } : item)
      .filter((item) => item.quantity > 0)
  })),
  remove: (dishPublicId) => set((state) => ({ items: state.items.filter((item) => item.dishPublicId !== dishPublicId) })),
  clear: () => set({ merchantPublicId: '', merchantName: '', items: [] }),
  totalQuantity: () => get().items.reduce((total, item) => total + item.quantity, 0),
  totalCents: () => get().items.reduce((total, item) => total + item.priceCents * item.quantity, 0)
}))
