# Web Research

Deep web research with query decomposition, multi-source search, and synthesized answers with source attribution.

## Usage

Ask me to research any topic. I will break your question into targeted searches, gather information from multiple sources, and synthesize a coherent answer with citations.

## Capabilities

- Decompose complex questions into targeted sub-queries
- Search across multiple sources and synthesize results
- Deduplicate overlapping information from different sources
- Assess confidence based on source freshness and authority
- Provide clear source attribution for all claims

## Research Methodology

### Step 1: Query Decomposition

Classify the question to determine search strategy:

| Query Type | Example | Strategy |
|-----------|---------|----------|
| **Factual** | "What is X?" | Direct search, authoritative sources |
| **Comparison** | "X vs Y?" | Search both, compare side-by-side |
| **How-to** | "How do I do X?" | Search tutorials, docs, guides |
| **Current events** | "What's happening with X?" | Recent results, news sources |
| **Exploratory** | "What do we know about X?" | Broad search, synthesize themes |

From the query, extract:
- **Keywords**: Core terms that must appear in results
- **Entities**: People, companies, projects, technologies
- **Constraints**: Time ranges, specific domains, formats
- **Intent**: What the user will do with this information

### Step 2: Multi-Source Search

Generate multiple query variants when the topic might be referred to differently:
```
User: "Kubernetes setup"
Queries: "Kubernetes", "k8s", "cluster setup", "container orchestration"
```

Search broadly, then narrow based on initial results.

### Step 3: Synthesis

Combine results into a coherent answer:

1. **Deduplicate** -- merge same info from different sources
2. **Cluster** -- group related results by theme
3. **Rank** -- order by relevance to the original question
4. **Assess confidence** -- freshness, authority, agreement across sources
5. **Synthesize** -- produce narrative answer with attribution

### Confidence Levels

- **High**: Multiple recent, authoritative sources agree
- **Moderate**: Single source or somewhat dated information
- **Low**: Old data, informal source, or conflicting signals -- flag explicitly

### Presenting Results

- Lead with the answer, not the search process
- Group by topic, not by source
- Surface conflicts explicitly rather than silently picking one version
- Include source links for verification
- Offer to go deeper on any sub-topic

## Example Prompts

- "Research the current state of WebAssembly adoption"
- "Compare Postgres vs MySQL for a new project"
- "What are the best practices for API rate limiting?"
- "Find information about company X's recent funding"

## Tools

- `web_search`: Search the web for information
- `web_fetch`: Fetch and read specific web pages
- `write_file`: Save research findings to a file
- `read_file`: Read existing research or reference material
