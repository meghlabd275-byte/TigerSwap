from setuptools import setup, find_packages

setup(
    name="tigerswap-ml",
    version="1.0.0",
    description="TigerSwap quantitative and machine-learning models",
    author="TigerSwap Team",
    packages=find_packages(),
    # Standard library only: the models are portable by design.
    install_requires=[],
    extras_require={"test": ["pytest>=7.0.0"]},
    python_requires=">=3.9",
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Programming Language :: Python :: 3.13",
    ],
)
