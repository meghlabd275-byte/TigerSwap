from setuptools import setup, find_packages

setup(
    name="tigerswap-sdk",
    version="1.0.0",
    description="TigerSwap Python SDK",
    author="TigerSwap Team",
    packages=find_packages(),
    install_requires=[
        "web3>=6.0.0",
        "eth-account>=0.9.0",
        "eth-typing>=4.0.0",
    ],
    python_requires=">=3.9",
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
)
