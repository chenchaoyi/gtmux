# frozen_string_literal: true
require 'json'
require 'time'
require 'tempfile'

# No tokens, upload-operation URLs, metadata values or private server data are recorded.
class StoreProgress
  class Pending < StandardError; end

  def initialize(journal: nil, clock: -> { Process.clock_gettime(Process::CLOCK_MONOTONIC) }, pause: ->(s) { sleep s })
    @journal, @clock, @pause = journal, clock, pause
    $stdout.sync = true
  end

  def event(phase, **fields)
    record = {time: Time.now.utc.iso8601, phase: phase, **fields}
    puts JSON.generate(record)
    return unless @journal
    flags = File::WRONLY | File::CREAT | File::APPEND | File::NOFOLLOW
    File.open(@journal, flags, 0o600) do |stream|
      raise 'Progress journal must be owned by the current user and mode 0600' unless stream.stat.uid == Process.euid && (stream.stat.mode & 0o077).zero?
      stream.puts(JSON.generate(record))
    end
  end

  def wait(id, timeout: 900)
    raise 'Processing wait must be 1..1800 seconds' unless timeout.between?(1, 1800)
    started = @clock.call
    state, reported = nil, -30
    loop do
      image = yield
      current = image.dig('attributes', 'state')
      elapsed = @clock.call - started
      if current != state || elapsed - reported >= 30
        event('apple-processing', image_id: id, state: current, elapsed_seconds: elapsed.to_i)
        state, reported = current, elapsed
      end
      return image unless %w[AWAITING_UPLOAD UPLOAD_COMPLETE].include?(current)
      if elapsed >= timeout
        event('pending', image_id: id, state: current)
        raise Pending, "Apple is still processing #{id}; rerun the same command to resume without reuploading."
      end
      @pause.call([5, timeout - elapsed].min)
    end
  end

  def self.save(path, data)
    raise 'State output must not be a symlink' if File.symlink?(path)
    Tempfile.create(['store-state', '.json'], File.dirname(File.expand_path(path))) do |stream|
      stream.write(JSON.pretty_generate(data) + "\n")
      stream.flush
      File.rename(stream.path, path)
    end
  end
end
