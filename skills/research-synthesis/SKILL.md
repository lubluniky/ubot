# Research Synthesis

Synthesize qualitative and quantitative research into structured insights. Analyze interview notes, survey responses, support tickets, or any unstructured data to identify themes and actionable findings.

## Usage

Share research data (interview notes, survey results, feedback, support tickets) and I will extract themes, identify patterns, and produce structured insights.

## Capabilities

- Thematic analysis of qualitative data (interviews, feedback, notes)
- Survey data interpretation (quantitative and open-ended)
- Cross-source triangulation for stronger findings
- Persona development from behavioral patterns
- Opportunity sizing and prioritization

## Thematic Analysis Method

1. **Familiarize**: Read through all data to get the overall landscape
2. **Code**: Tag each observation, quote, or data point with descriptive codes
3. **Develop themes**: Group related codes into candidate themes
4. **Review**: Check themes against data -- sufficient evidence? Distinct from each other?
5. **Refine**: Define and name each theme clearly with 1-2 sentence description
6. **Report**: Write up themes as findings with supporting evidence

## Extracting Insights from Notes

For each source, identify:

- **Observations**: What was described, experienced, or felt? Note context (when, where, how often)
- **Direct quotes**: Verbatim statements that powerfully illustrate a point
- **Behaviors vs. stated preferences**: What people DO often differs from what they SAY
- **Signals of intensity**: Emotional language, frequency of issue, workarounds, impact when things go wrong

## Cross-Source Analysis

- Look for patterns that appear across multiple sources
- Note frequency: how many sources mention each theme
- Identify segments: do different groups show different patterns
- Surface contradictions: disagreements often reveal meaningful segments
- Find surprises: what challenges prior assumptions

## Triangulation

Strengthen findings by combining:
- **Method triangulation**: Same question, different methods (interviews + survey + analytics)
- **Source triangulation**: Same method, different participants
- **Temporal triangulation**: Same observation at different points in time

A finding supported by multiple sources is much stronger than one from a single source.

## Presenting Findings

### Insight Format
```
**Theme: [Name]**
[1-2 sentence description of what this theme captures]

Evidence:
- [Observation 1 with source attribution]
- [Observation 2 with source attribution]
- [Supporting quote]

Frequency: Mentioned by N of M sources
Confidence: High/Medium/Low
Implication: [What this means for decisions]
```

### When Sources Disagree
Report the disagreement honestly. Check if it is due to different populations, stated vs. actual preferences, or measurement differences. Investigate further rather than silently picking one version.

## Example Prompts

- "Analyze these interview notes and find the key themes"
- "Synthesize this customer feedback into actionable insights"
- "What patterns do you see in these support tickets?"
- "Combine these survey results with the interview findings"
- "Build user personas from this research data"

## Tools

- `read_file`: Read research data, notes, and survey results
- `write_file`: Save synthesis reports and findings
- `edit_file`: Update existing research documents
- `list_dir`: Find research files in a directory
- `web_search`: Research industry benchmarks or related findings
