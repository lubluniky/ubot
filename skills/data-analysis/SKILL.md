# Data Analysis

Profile, explore, validate, and visualize datasets. Systematic methodology for understanding data quality, discovering patterns, and producing reliable analysis.

## Usage

Share a dataset or point me to data files, and I will help you explore, validate, and analyze them.

## Capabilities

- Profile datasets to understand shape, quality, and patterns
- Detect data quality issues (nulls, duplicates, inconsistencies)
- Statistical analysis and distribution characterization
- Validate analysis results before sharing with stakeholders
- Create visualizations with Python (matplotlib, seaborn, plotly)

## Data Profiling Methodology

### Phase 1: Structural Understanding

Before analyzing any data, understand its structure:

**Table-level questions:**
- How many rows and columns?
- What is the grain (one row per what)?
- What is the primary key? Is it unique?
- When was the data last updated?

**Column classification:**
- **Identifier**: Unique keys, foreign keys, entity IDs
- **Dimension**: Categorical attributes for grouping/filtering
- **Metric**: Quantitative values for measurement
- **Temporal**: Dates and timestamps
- **Text**: Free-form text fields
- **Boolean**: True/false flags

### Phase 2: Column-Level Profiling

For each column, compute:

**All columns:** Null count/rate, distinct count, most common values (top 5-10)

**Numeric columns:** min, max, mean, median, standard deviation, percentiles (p1, p5, p25, p75, p95, p99)

**String columns:** min/max/avg length, empty string count, pattern analysis

**Date columns:** min/max date, gaps in time series, distribution by period

### Phase 3: Pattern Discovery

- **Distribution analysis**: Normal, skewed, bimodal, power law, uniform
- **Temporal patterns**: Trend, seasonality, day-of-week effects, change points
- **Correlations**: Flag strong correlations (|r| > 0.7) for investigation

## Quality Assessment

### Completeness Score

- **Complete** (>99% non-null): Good to use
- **Mostly complete** (95-99%): Investigate the nulls
- **Incomplete** (80-95%): Understand why, decide if usable
- **Sparse** (<80%): May need imputation or exclusion

### Common Pitfalls to Check

- **Join explosion**: Many-to-many joins silently multiplying rows
- **Survivorship bias**: Analyzing only entities that exist today
- **Incomplete period comparison**: Comparing partial to full periods
- **Average of averages**: Wrong when group sizes differ
- **Timezone mismatches**: Different sources using different timezones

## Pre-Delivery QA Checklist

Before sharing any analysis:

- [ ] Source tables verified and data is fresh enough
- [ ] No unexpected gaps or missing segments
- [ ] Nulls handled appropriately
- [ ] No double-counting from bad joins
- [ ] Aggregation logic matches the analysis grain
- [ ] Rates use correct denominators
- [ ] Numbers are in a plausible range
- [ ] Key numbers cross-referenced against known sources
- [ ] Charts start at zero (bar charts), axes labeled
- [ ] Assumptions and limitations stated explicitly

## Example Prompts

- "Analyze this CSV file and tell me what you find"
- "Check this data for quality issues before I share it"
- "Create a visualization showing the trend over time"
- "Profile this dataset and document the schema"

## Tools

- `read_file`: Read data files (CSV, JSON, etc.)
- `write_file`: Save analysis results and documentation
- `exec`: Run Python scripts for statistical analysis and visualization
- `list_dir`: Explore data file locations
