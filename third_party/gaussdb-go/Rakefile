require "erb"

rule '.go' => '.go.erb' do |task|
  erb = ERB.new(File.read(task.source))
  File.write(task.name, "// Code generated from #{task.source}. DO NOT EDIT.\n\n" + erb.result(binding))
  sh "goimports", "-w", task.name
end

generated_code_files = [
  "gaussdbtype/int.go",
  "gaussdbtype/int_test.go",
  "gaussdbtype/integration_benchmark_test.go",
  "gaussdbtype/zeronull/int.go",
  "gaussdbtype/zeronull/int_test.go"
]

desc "Generate code"
task generate: generated_code_files
