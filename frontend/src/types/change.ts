export type ChangeType = 'infrastructure' | 'deployment' | 'configuration';
export type Environment = 'production' | 'staging' | 'preprod' | 'development' | string;

export interface Change {
  id: string;
  system: string;
  environment?: string;
  user?: string;
  type: ChangeType;
  description: string;
  timestamp: string; // ISO 8601
}

export interface ChangeFilters {
  system?: string;
  environment?: string;
  user?: string;
  type?: ChangeType | '';
  timeRange?: '30m' | '1h' | '2h' | '6h' | '24h' | '7d' | 'custom' | '';
  customFrom?: string; // ISO datetime-local string
  customTo?: string;   // ISO datetime-local string
  search?: string;
}

export const CHANGE_TYPE_LABELS: Record<ChangeType, string> = {
  infrastructure: 'Infrastructure',
  deployment: 'Deployment',
  configuration: 'Configuration',
};

export const CHANGE_TYPE_COLORS: Record<ChangeType, string> = {
  infrastructure: 'badge-infra',
  deployment: 'badge-deploy',
  configuration: 'badge-config',
};

export const KNOWN_SYSTEMS = [
  'api-gateway',
  'auth-service',
  'billing-service',
  'cdn',
  'database-primary',
  'database-replica',
  'frontend',
  'kafka',
  'kubernetes-cluster',
  'message-queue',
  'monitoring',
  'payment-service',
  'redis-cache',
  'search-service',
  'storage-service',
  'user-service',
];

export const KNOWN_ENVIRONMENTS: Environment[] = [
  'production',
  'staging',
  'preprod',
  'development',
];
