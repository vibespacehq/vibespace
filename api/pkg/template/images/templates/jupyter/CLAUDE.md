# Jupyter Lab 4.4 + Python 3.12 Vibespace

## Installed Versions (October 2025)
- **Python**: 3.12.3 (Ubuntu 24.04 default)
- **Jupyter Lab**: 4.4.9
- **NumPy**: latest
- **Pandas**: latest
- **Matplotlib**: latest
- **Seaborn**: latest
- **Scikit-learn**: latest
- **SciPy**: latest
- **Plotly**: latest

## Features
- **Jupyter Lab 4.4** interface (web-based)
- **IPython kernel**
- **Interactive widgets** (ipywidgets)
- **Rich output** (plots, tables, HTML)
- **Markdown support**
- **Code completion**

## Ports
- **code-server**: 8080 (VS Code)
- **Jupyter Lab**: 8888 (notebooks)

## Access
- Code-server: `http://vibespace-{id}.local`
- Jupyter Lab: `http://vibespace-{id}-8888.local`

## Commands
- **Jupyter**: Already running in background
- **Install packages**: `pip3 install --user <package>`
- **List packages**: `pip3 list`

## Best Practices
- ✅ Descriptive notebook names (`01-data-exploration.ipynb`)
- ✅ Add markdown cells for documentation
- ✅ Clear outputs before committing
- ✅ Keep notebooks focused
- ✅ Save data to `/vibespace/data/`
- ✅ Export plots to `/vibespace/outputs/`

## Project Structure
```
notebooks/              # Jupyter notebooks
├── 01-exploration.ipynb
├── 02-preprocessing.ipynb
└── 03-modeling.ipynb
data/                  # Datasets
├── raw/              # Original data
└── processed/        # Cleaned data
scripts/              # Python scripts
outputs/              # Plots, reports
requirements.txt      # Dependencies
```

## Popular Packages (install as needed)
- **Polars**: `pip install --user polars`
- **XGBoost**: `pip install --user xgboost`
- **PyTorch**: `pip install --user torch`
- **TensorFlow**: `pip install --user tensorflow`

## Resources
- [Python 3.12 What's New](https://docs.python.org/3.12/whatsnew/3.12.html)
- [Jupyter Lab Docs](https://jupyterlab.readthedocs.io/)
- [Pandas Docs](https://pandas.pydata.org/docs/)
