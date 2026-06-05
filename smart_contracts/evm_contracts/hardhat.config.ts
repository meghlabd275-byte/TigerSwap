/**
 * TigerSwap Hardhat Configuration
 * Complete deployment and testing configuration for all smart contracts
 */

import { HardhatUserConfig } from "hardhat/types";
import "@nomicfoundation/hardhat-toolbox";
import "@nomicfoundation/hardhat-ethers";
import "hardhat-deploy";
import "hardhat-gas-reporter";
import "solidity-coverage";
import * as dotenv from "dotenv";

dotenv.config();

const PRIVATE_KEY = process.env.PRIVATE_KEY || "0x0000000000000000000000000000000000000000000000000000000000000000";
const ETHERSCAN_API_KEY = process.env.ETHERSCAN_API_KEY || "";
const POLYGONSCAN_API_KEY = process.env.POLYGONSCAN_API_KEY || "";
const ARBISCAN_API_KEY = process.env.ARBISCAN_API_KEY || "";
const BSCSCAN_API_KEY = process.env.BSCSCAN_API_KEY || "";

const FOUNDRY_PROFILE = process.env.FOUNDRY_PROFILE || "default";

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.20",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
      viaIR: false,
    },
  },
  
  networks: {
    hardhat: {
      chainId: 31337,
      forking: process.env.MAINNET_RPC_URL ? {
        url: process.env.MAINNET_RPC_URL,
        blockNumber: parseInt(process.env.FORKING_BLOCK_NUMBER || "0"),
      } : undefined,
      accounts: {
        mnemonic: "test test test test test test test test test test test junk",
        count: 20,
        path: "m/44'/60'/0'/0",
      },
    },
    
    localhost: {
      url: "http://127.0.0.1:8545",
      chainId: 31337,
    },
    
    mainnet: {
      url: process.env.MAINNET_RPC_URL || "https://eth.llamarpc.com",
      chainId: 1,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    sepolia: {
      url: process.env.SEPOLIA_RPC_URL || "https://rpc.sepolia.org",
      chainId: 11155111,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    arbitrum: {
      url: process.env.ARBITRUM_RPC_URL || "https://arb1.arbitrum.io/rpc",
      chainId: 42161,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    arbitrumSepolia: {
      url: process.env.ARB_SEPOLIA_RPC_URL || "https://sepolia-rollup.arbitrum.io/rpc",
      chainId: 421614,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    polygon: {
      url: process.env.POLYGON_RPC_URL || "https://polygon-rpc.com",
      chainId: 137,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    polygonMumbai: {
      url: process.env.MUMBAI_RPC_URL || "https://rpc-mumbai.maticvigil.com",
      chainId: 80001,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    bsc: {
      url: process.env.BSC_RPC_URL || "https://bsc-dataseed.binance.org",
      chainId: 56,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    bscTestnet: {
      url: process.env.BSC_TESTNET_RPC_URL || "https://data-seed-prebsc-1-s1.binance.org:8545",
      chainId: 97,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    optimism: {
      url: process.env.OPTIMISM_RPC_URL || "https://mainnet.optimism.io",
      chainId: 10,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    base: {
      url: process.env.BASE_RPC_URL || "https://mainnet.base.org",
      chainId: 8453,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    baseSepolia: {
      url: process.env.BASE_SEPOLIA_RPC_URL || "https://sepolia.base.org",
      chainId: 84532,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    avalanche: {
      url: process.env.AVALANCHE_RPC_URL || "https://api.avax.network/ext/bc/C/rpc",
      chainId: 43114,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
    
    fantom: {
      url: process.env.FANTOM_RPC_URL || "https://rpc.fantom.network",
      chainId: 250,
      accounts: [PRIVATE_KEY],
      gasPrice: "auto",
    },
  },
  
  etherscan: {
    apiKey: {
      mainnet: ETHERSCAN_API_KEY,
      sepolia: ETHERSCAN_API_KEY,
      polygon: POLYGONSCAN_API_KEY,
      polygonMumbai: POLYGONSCAN_API_KEY,
      arbitrumOne: ARBISCAN_API_KEY,
      arbitrumSepolia: ARBISCAN_API_KEY,
      bsc: BSCSCAN_API_KEY,
      bscTestnet: BSCSCAN_API_KEY,
      optimisticEthereum: ETHERSCAN_API_KEY,
      base: ETHERSCAN_API_KEY,
      baseSepolia: ETHERSCAN_API_KEY,
      avalanche: ETHERSCAN_API_KEY,
      avalancheFuji: ETHERSCAN_API_KEY,
      fantom: ETHERSCAN_API_KEY,
      fantomTestnet: ETHERSCAN_API_KEY,
    },
    customChains: [],
  },
  
  gasReporter: {
    enabled: process.env.REPORT_GAS === "true",
    currency: process.env.GAS_REPORT_CURRENCY || "USD",
    coinmarketcap: process.env.COINMARKETCAP_API_KEY || "",
    token: "ETH",
    excludeContracts: [],
    src: "./contracts",
  },
  
  paths: {
    sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
    deploy: "./scripts",
    deployments: "./deployments",
  },
  
  namedAccounts: {
    deployer: {
      default: 0,
    },
    feeRecipient: {
      default: 1,
    },
    governance: {
      default: 2,
    },
    team: {
      default: 3,
    },
    investor: {
      default: 4,
    },
  },
  
  external: {
    deployments: {
      hardhat: ["./deployments/hardhat"],
      localhost: ["./deployments/localhost"],
    },
  },
  
  mocha: {
    timeout: 100000,
    retries: 2,
  },
};

export default config;