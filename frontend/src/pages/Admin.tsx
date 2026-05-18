import Layout from '@/components/Layout';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Shield, Users, Key, Activity } from 'lucide-react';
import SsoTab from '@/components/admin/SsoTab';
import UsersTab from '@/components/admin/UsersTab';
import ApiKeysTab from '@/components/admin/ApiKeysTab';
import AuditTab from '@/components/admin/AuditTab';

const Admin = () => {
  return (
    <Layout>
      <div>
        {/* Header */}
        <div className="mb-6">
          <div className="flex items-center gap-2 mb-1">
            <Shield className="w-4 h-4 text-primary" />
            <h1 className="text-lg font-semibold text-foreground">Access & Security</h1>
          </div>
          <p className="text-sm text-muted-foreground">
            Manage authentication, users, API keys, and audit logs. Admin access only.
          </p>
        </div>

        <Tabs defaultValue="users">
          <TabsList className="bg-secondary mb-6 h-auto p-1 gap-1 w-full">
            {[
              { value: 'users', label: 'Users', icon: Users },
              { value: 'sso', label: 'SSO', icon: Shield },
              { value: 'api-keys', label: 'API Keys', icon: Key },
              { value: 'audit', label: 'Audit Log', icon: Activity },
            ].map(({ value, label, icon: Icon }) => (
              <TabsTrigger
                key={value}
                value={value}
                className="flex-1 gap-1.5 text-sm px-4 py-2 data-[state=active]:bg-card data-[state=active]:text-foreground data-[state=active]:shadow-sm"
              >
                <Icon className="w-3.5 h-3.5" />
                {label}
              </TabsTrigger>
            ))}
          </TabsList>

          <TabsContent value="sso"><SsoTab /></TabsContent>
          <TabsContent value="users"><UsersTab /></TabsContent>
          <TabsContent value="api-keys"><ApiKeysTab /></TabsContent>
          <TabsContent value="audit"><AuditTab /></TabsContent>
        </Tabs>
      </div>
    </Layout>
  );
};

export default Admin;
