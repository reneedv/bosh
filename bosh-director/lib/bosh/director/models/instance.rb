module Bosh::Director::Models
  class Instance < Sequel::Model(Bosh::Director::Config.db)
    many_to_one :deployment
    many_to_one :vm
    one_to_many :persistent_disks
    one_to_many :rendered_templates_archives

    def validate
      validates_presence [:deployment_id, :job, :index, :state]
      validates_unique [:deployment_id, :job, :index]
      validates_unique [:vm_id] if vm_id
      validates_integer :index
      validates_includes ["started", "stopped", "detached"], :state
    end

    def persistent_disk
      # Currently we support only 1 persistent disk.
      self.persistent_disks.find { |disk| disk.active }
    end

    def persistent_disk_cid
      disk = persistent_disk
      return disk.disk_cid if disk
      nil
    end

    def latest_rendered_templates_archive
      rendered_templates_archives_dataset.order(:created_at).last
    end

    def stale_rendered_templates_archives
      stale_archives = rendered_templates_archives_dataset
      if latest = latest_rendered_templates_archive
        stale_archives.exclude(id: latest.id)
      else
        stale_archives
      end
    end
  end
end
