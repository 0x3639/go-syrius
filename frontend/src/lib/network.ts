// Names for the chains the wallet knows. Anything else is shown by its number
// so an unfamiliar network is never mistaken for a known one.
export const MAINNET_CHAIN_ID = 1
export const TESTNET_CHAIN_ID = 73404

export function networkName(chainId: number): string {
  if (chainId === MAINNET_CHAIN_ID) return 'Mainnet'
  if (chainId === TESTNET_CHAIN_ID) return 'Testnet'
  return 'Custom network'
}
