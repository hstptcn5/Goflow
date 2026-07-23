import aiAssistant from '../../../templates/workflow_ai_assistant.json';
import githubRepoMonitor from '../../../templates/github_repo_monitor.json';
import weatherAlert from '../../../templates/weather_alert_flow.json';
import multiBranchStressTest from '../../../templates/multi_branch_stress_test.json';
import uptimeIncidentResponse from '../../../templates/uptime_incident_response.json';
import customerSupportAiTriage from '../../../templates/customer_support_ai_triage.json';
import releaseSmokeTest from '../../../templates/release_smoke_test.json';

export const workflowTemplates = [
  {
    id: 'customer-support-ai-triage',
    title: 'Customer Support AI Triage',
    category: 'AI + Support',
    difficulty: 'Advanced',
    summary: 'Classify support tickets with AI, then route urgent and normal cases to different channels.',
    requirements: ['DeepSeek credential', 'Telegram chat ID', 'Slack webhook'],
    workflow: customerSupportAiTriage,
  },
  {
    id: 'uptime-incident-response',
    title: 'Uptime Incident Response',
    category: 'Monitoring',
    difficulty: 'Intermediate',
    summary: 'Check a health endpoint, store status in Redis, and alert Discord when the endpoint is unhealthy.',
    requirements: ['Health URL', 'Redis server', 'Discord webhook'],
    workflow: uptimeIncidentResponse,
  },
  {
    id: 'release-smoke-test',
    title: 'Release Smoke Test',
    category: 'DevOps',
    difficulty: 'Advanced',
    summary: 'Pull code, restart a remote service, run a public health check, and notify success or failure.',
    requirements: ['Git repo path', 'SSH credential', 'Telegram or Discord'],
    workflow: releaseSmokeTest,
  },
  {
    id: 'weather-alert',
    title: 'Weather Alert Flow',
    category: 'API Automation',
    difficulty: 'Beginner',
    summary: 'Fetch live weather data from Open-Meteo and branch based on fetch status.',
    requirements: ['Internet access'],
    workflow: weatherAlert,
  },
  {
    id: 'github-repo-monitor',
    title: 'GitHub Repo Monitor',
    category: 'Developer Tools',
    difficulty: 'Beginner',
    summary: 'Poll GitHub API data and process repository status on a schedule.',
    requirements: ['GitHub API URL'],
    workflow: githubRepoMonitor,
  },
  {
    id: 'ai-assistant-pipeline',
    title: 'AI Text Pipeline',
    category: 'AI',
    difficulty: 'Beginner',
    summary: 'Receive a webhook, prepare a prompt, call DeepSeek, and format the result.',
    requirements: ['DeepSeek API key'],
    workflow: aiAssistant,
  },
  {
    id: 'multi-branch-stress-test',
    title: 'Multi-Branch Stress Test',
    category: 'Testing',
    difficulty: 'Intermediate',
    summary: 'Exercise parallel workflow branches with multiple HTTP requests.',
    requirements: ['Internet access'],
    workflow: multiBranchStressTest,
  },
];

