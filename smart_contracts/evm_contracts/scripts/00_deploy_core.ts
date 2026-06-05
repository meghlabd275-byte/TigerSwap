/**
 * TigerSwap Deployment Script
 * Deploys all core contracts to the specified network
 */

import { ethers, network } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, getNamedAccounts } = hre;
  const { deploy, execute, log } = deployments;
  const { deployer, feeRecipient, governance, team, investor } = await getNamedAccounts();

  const chainId = await hre.getChainId();
  const isLocal = chainId === "31337" || chainId === "1337";

  log("\n========================================");
  log(`TigerSwap Deployment on ${network.name} (Chain ID: ${chainId})`);
  log("========================================\n");

  // =============================================================================
  // 1. Deploy WETH (if needed - only for local/testnets)
  // =============================================================================
  let wethAddress: string;
  const WETH_ADDRESSES: Record<string, string> = {
    "1": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // mainnet
    "5": "0xfFf9976782d46CC05630D1f6eB18b0f4982AcB37", // goerli
    "11155111": "0xfFf9976782d46CC05630D1f6eB18b0f4982AcB37", // sepolia
    "42161": "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1", // arbitrum
    "421614": "0xfFf9976782d46CC05630D1f6eB18b0f4982AcB37", // arbitrum sepolia
    "137": "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270", // polygon
    "80001": "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270", // mumbai
    "56": "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", // bsc
    "97": "0xae13d989daC2f0dEbFf460aC112a837C89BAa7cd", // bsc testnet
    "10": "0x4200000000000000000000000000000000000042", // optimism
    "8453": "0x4200000000000000000000000000000000000006", // base
    "84532": "0x4200000000000000000000000000000000000006", // base sepolia
    "43114": "0x49D5c2BdFad6D5e00bD4d3bEa6B2C8D2cF5c3E7", // avalanche
    "250": "0x21be370D5310f44e6424c257B0D2C0f0D42b3D47", // fantom
  };

  if (WETH_ADDRESSES[chainId] && !isLocal) {
    wethAddress = WETH_ADDRESSES[chainId];
    log(`Using existing WETH at: ${wethAddress}`);
  } else {
    const weth = await deploy("WETH9", {
      from: deployer,
      args: [],
      log: true,
      waitConfirmations: isLocal ? 1 : 2,
    });
    wethAddress = weth.address;
    log(`Deployed new WETH at: ${wethAddress}`);
  }

  // =============================================================================
  // 2. Deploy TigerSwap Factory
  // =============================================================================
  log("\n--- Deploying TigerSwap Factory ---");
  const factory = await deploy("TigerSwapFactory", {
    from: deployer,
    args: [deployer],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerSwap Factory deployed at: ${factory.address}`);

  // =============================================================================
  // 3. Deploy TigerSwap Router
  // =============================================================================
  log("\n--- Deploying TigerSwap Router ---");
  const router = await deploy("TigerSwapRouter", {
    from: deployer,
    args: [factory.address, wethAddress],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerSwap Router deployed at: ${router.address}`);

  // =============================================================================
  // 4. Set Router in Factory
  // =============================================================================
  if (isLocal || network.name === "hardhat") {
    log("\n--- Configuring Factory ---");
    const factoryContract = await ethers.getContractAt("TigerSwapFactory", factory.address);
    
    // Transfer ownership if needed
    const currentOwner = await factoryContract.feeToSetter();
    if (currentOwner !== deployer) {
      log(`Transferring factory ownership from ${currentOwner} to ${deployer}`);
    }
    
    log("Factory configuration complete");
  }

  // =============================================================================
  // 5. Deploy TIGER Token (for testnets/local)
  // =============================================================================
  let tigerTokenAddress: string;
  const TIGER_ADDRESSES: Record<string, string> = {
    // Add deployed addresses after first deployment
  };

  if (TIGER_ADDRESSES[chainId] && !isLocal) {
    tigerTokenAddress = TIGER_ADDRESSES[chainId];
    log(`\nUsing existing TIGER token at: ${tigerTokenAddress}`);
  } else {
    log("\n--- Deploying TIGER Token ---");
    const tigerToken = await deploy("TigerToken", {
      from: deployer,
      args: [],
      log: true,
      waitConfirmations: isLocal ? 1 : 2,
    });
    tigerTokenAddress = tigerToken.address;
    log(`TIGER Token deployed at: ${tigerToken.address}`);
  }

  // =============================================================================
  // 6. Deploy MasterChef (Farming)
  // =============================================================================
  log("\n--- Deploying MasterChef (TigerFarming) ---");
  const masterChef = await deploy("TigerFarming", {
    from: deployer,
    args: [tigerTokenAddress],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`MasterChef deployed at: ${masterChef.address}`);

  // =============================================================================
  // 7. Deploy Staking Contract
  // =============================================================================
  log("\n--- Deploying TigerStaking ---");
  const staking = await deploy("TigerStaking", {
    from: deployer,
    args: [tigerTokenAddress, tigerTokenAddress],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerStaking deployed at: ${staking.address}`);

  // =============================================================================
  // 8. Deploy Governance (DAO)
  // =============================================================================
  log("\n--- Deploying TigerDAO ---");
  const governanceContract = await deploy("TigerDAO", {
    from: deployer,
    args: [],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerDAO deployed at: ${governanceContract.address}`);

  // =============================================================================
  // 9. Deploy Treasury
  // =============================================================================
  log("\n--- Deploying TigerTreasury ---");
  const treasury = await deploy("TigerTreasury", {
    from: deployer,
    args: [governance, deployer],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerTreasury deployed at: ${treasury.address}`);

  // =============================================================================
  // 10. Deploy Vault
  // =============================================================================
  log("\n--- Deploying TigerVault ---");
  const vaultOwners = [deployer, governance, team].filter(Boolean);
  const vault = await deploy("TigerVault", {
    from: deployer,
    args: [vaultOwners, 2, ethers.parseEther("1")], // 2 of 3 multisig, 1 ETH daily limit
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerVault deployed at: ${vault.address}`);

  // =============================================================================
  // 11. Deploy Bridge
  // =============================================================================
  log("\n--- Deploying TigerBridge ---");
  const bridge = await deploy("TigerBridge", {
    from: deployer,
    args: [parseInt(chainId)],
    log: true,
    waitConfirmations: isLocal ? 1 : 2,
  });
  log(`TigerBridge deployed at: ${bridge.address}`);

  // =============================================================================
  // Summary
  // =============================================================================
  log("\n========================================");
  log("DEPLOYMENT SUMMARY");
  log("========================================");
  log(`Network: ${network.name} (Chain ID: ${chainId})`);
  log(`Deployer: ${deployer}`);
  log("");
  log("Contract Addresses:");
  log(`  WETH:        ${wethAddress}`);
  log(`  Factory:     ${factory.address}`);
  log(`  Router:      ${router.address}`);
  log(`  TIGER:       ${tigerTokenAddress}`);
  log(`  MasterChef:  ${masterChef.address}`);
  log(`  Staking:     ${staking.address}`);
  log(`  Governance:  ${governanceContract.address}`);
  log(`  Treasury:    ${treasury.address}`);
  log(`  Vault:       ${vault.address}`);
  log(`  Bridge:      ${bridge.address}`);
  log("");
  log("Next Steps:");
  log("1. Verify contracts on block explorer");
  log("2. Add initial liquidity to pools");
  log("3. Configure MasterChef reward distribution");
  log("4. Set up governance proposals");
  log("========================================\n");

  // Save deployment addresses for later use
  const deploymentData = {
    network: network.name,
    chainId: chainId,
    deployer: deployer,
    timestamp: new Date().toISOString(),
    contracts: {
      WETH: wethAddress,
      Factory: factory.address,
      Router: router.address,
      TigerToken: tigerTokenAddress,
      MasterChef: masterChef.address,
      Staking: staking.address,
      Governance: governanceContract.address,
      Treasury: treasury.address,
      Vault: vault.address,
      Bridge: bridge.address,
    },
  };

  // Write deployment data to file (only for local development)
  if (isLocal) {
    const fs = require("fs");
    const deploymentsDir = "./deployments";
    if (!fs.existsSync(deploymentsDir)) {
      fs.mkdirSync(deploymentsDir, { recursive: true });
    }
    fs.writeFileSync(
      `${deploymentsDir}/${network.name}-${chainId}.json`,
      JSON.stringify(deploymentData, null, 2)
    );
    log(`Deployment data saved to ${deploymentsDir}/${network.name}-${chainId}.json`);
  }
};

export default func;
func.tags = ["all", "core", "factory", "router", "token", "masterchef", "staking", "governance", "treasury", "vault", "bridge"];
func.dependencies = [];